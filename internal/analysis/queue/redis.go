package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultConsumeTimeout = 5 * time.Second

type RedisQueue struct {
	client        redis.Cmdable
	streamKey     string
	groupName     string
	consumerName  string
	delayedKey    string
	deadLetterKey string
	lockKeyPrefix string
	claimMinIdle  time.Duration
}

func NewRedisQueue(client redis.Cmdable, name string) (*RedisQueue, error) {
	if client == nil {
		return nil, errors.New("redis client is required")
	}
	if name == "" {
		return nil, errors.New("queue name is required")
	}

	streamKey := fmt.Sprintf("stream:%s", name)
	return &RedisQueue{
		client:        client,
		streamKey:     streamKey,
		groupName:     fmt.Sprintf("%s-group", name),
		consumerName:  fmt.Sprintf("%s-consumer", name),
		delayedKey:    fmt.Sprintf("stream:%s:delayed", name),
		deadLetterKey: fmt.Sprintf("stream:%s:dead-letter", name),
		lockKeyPrefix: fmt.Sprintf("stream:%s:chat-lock:", name),
		claimMinIdle:  time.Minute,
	}, nil
}

func NewRedisClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func (q *RedisQueue) Enqueue(ctx context.Context, job AnalysisJob) error {
	payload, err := encodeJob(job)
	if err != nil {
		return err
	}

	if err := q.ensureGroup(ctx); err != nil {
		return err
	}

	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.streamKey,
		Values: map[string]interface{}{
			"payload": payload,
		},
	}).Err()
}

func (q *RedisQueue) Consume(ctx context.Context, timeout time.Duration) (ConsumedJob, error) {
	if timeout <= 0 {
		timeout = defaultConsumeTimeout
	}

	if err := q.ensureGroup(ctx); err != nil {
		return ConsumedJob{}, err
	}

	reclaimed, err := q.reclaimStale(ctx)
	if err != nil {
		return ConsumedJob{}, err
	}
	if len(reclaimed) > 0 {
		return consumedFromMessage(reclaimed[0])
	}

	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.groupName,
		Consumer: q.consumerName,
		Streams:  []string{q.streamKey, ">"},
		Count:    1,
		Block:    timeout,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ConsumedJob{}, ErrNoJob
		}

		return ConsumedJob{}, err
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return ConsumedJob{}, ErrNoJob
	}

	return consumedFromMessage(streams[0].Messages[0])
}

func (q *RedisQueue) Ack(ctx context.Context, consumed ConsumedJob) error {
	if consumed.streamID == "" {
		return errors.New("consumed stream id is required")
	}

	pipe := q.client.TxPipeline()
	pipe.XAck(ctx, q.streamKey, q.groupName, consumed.streamID)
	pipe.XDel(ctx, q.streamKey, consumed.streamID)
	_, err := pipe.Exec(ctx)
	return err
}

func (q *RedisQueue) RetryLater(ctx context.Context, consumed ConsumedJob, runAt time.Time) error {
	if consumed.streamID == "" {
		return errors.New("consumed stream id is required")
	}

	job := consumed.Job
	job.Attempt++
	payload, err := encodeJob(job)
	if err != nil {
		return err
	}

	pipe := q.client.TxPipeline()
	pipe.XAck(ctx, q.streamKey, q.groupName, consumed.streamID)
	pipe.XDel(ctx, q.streamKey, consumed.streamID)
	pipe.ZAdd(ctx, q.delayedKey, redis.Z{
		Score:  float64(runAt.UnixMilli()),
		Member: payload,
	})
	_, err = pipe.Exec(ctx)
	return err
}

func (q *RedisQueue) DeadLetter(ctx context.Context, consumed ConsumedJob) error {
	if consumed.streamID == "" {
		return errors.New("consumed stream id is required")
	}

	pipe := q.client.TxPipeline()
	pipe.XAck(ctx, q.streamKey, q.groupName, consumed.streamID)
	pipe.XDel(ctx, q.streamKey, consumed.streamID)
	pipe.LPush(ctx, q.deadLetterKey, consumed.raw)
	_, err := pipe.Exec(ctx)
	return err
}

func (q *RedisQueue) AcquireChatLock(ctx context.Context, chatID, owner string, ttl time.Duration) (bool, error) {
	if chatID == "" {
		return false, errors.New("chat id is required")
	}
	if owner == "" {
		return false, errors.New("lock owner is required")
	}
	if ttl <= 0 {
		ttl = time.Minute
	}

	return q.client.SetNX(ctx, q.chatLockKey(chatID), owner, ttl).Result()
}

func (q *RedisQueue) ReleaseChatLock(ctx context.Context, chatID, owner string) error {
	if chatID == "" || owner == "" {
		return nil
	}

	script := redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)
	return script.Run(ctx, q.client, []string{q.chatLockKey(chatID)}, owner).Err()
}

func (q *RedisQueue) MoveDueRetries(ctx context.Context, now time.Time, limit int64) (int64, error) {
	if limit <= 0 {
		limit = 100
	}

	payloads, err := q.client.ZRangeByScore(ctx, q.delayedKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%d", now.UnixMilli()),
		Count: limit,
	}).Result()
	if err != nil {
		return 0, err
	}
	if len(payloads) == 0 {
		return 0, nil
	}

	pipe := q.client.TxPipeline()
	for _, payload := range payloads {
		pipe.ZRem(ctx, q.delayedKey, payload)
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: q.streamKey,
			Values: map[string]interface{}{
				"payload": payload,
			},
		})
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}

	return int64(len(payloads)), nil
}

func (q *RedisQueue) chatLockKey(chatID string) string {
	return q.lockKeyPrefix + chatID
}

func (q *RedisQueue) ensureGroup(ctx context.Context) error {
	err := q.client.XGroupCreateMkStream(ctx, q.streamKey, q.groupName, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}

	return nil
}

func (q *RedisQueue) reclaimStale(ctx context.Context) ([]redis.XMessage, error) {
	messages, _, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   q.streamKey,
		Group:    q.groupName,
		Consumer: q.consumerName,
		MinIdle:  q.claimMinIdle,
		Start:    "0-0",
		Count:    1,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}

		return nil, err
	}

	return messages, nil
}

func consumedFromMessage(message redis.XMessage) (ConsumedJob, error) {
	raw, ok := message.Values["payload"].(string)
	if !ok || raw == "" {
		return ConsumedJob{}, errors.New("stream message payload is required")
	}

	job, err := decodeJob(raw)
	if err != nil {
		return ConsumedJob{}, err
	}

	return ConsumedJob{Job: job, raw: raw, streamID: message.ID}, nil
}

var ErrNoJob = errors.New("no analysis job available")

func encodeJob(job AnalysisJob) (string, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return "", err
	}

	return string(payload), nil
}

func decodeJob(payload string) (AnalysisJob, error) {
	var job AnalysisJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return AnalysisJob{}, err
	}

	return job, nil
}
