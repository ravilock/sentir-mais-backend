package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultConsumeTimeout = 5 * time.Second

type RedisQueue struct {
	client        redis.Cmdable
	readyKey      string
	processingKey string
	delayedKey    string
	deadLetterKey string
	lockKeyPrefix string
}

func NewRedisQueue(client redis.Cmdable, name string) (*RedisQueue, error) {
	if client == nil {
		return nil, errors.New("redis client is required")
	}
	if name == "" {
		return nil, errors.New("queue name is required")
	}

	return &RedisQueue{
		client:        client,
		readyKey:      fmt.Sprintf("queue:%s:ready", name),
		processingKey: fmt.Sprintf("queue:%s:processing", name),
		delayedKey:    fmt.Sprintf("queue:%s:delayed", name),
		deadLetterKey: fmt.Sprintf("queue:%s:dead-letter", name),
		lockKeyPrefix: fmt.Sprintf("queue:%s:chat-lock:", name),
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

	return q.client.LPush(ctx, q.readyKey, payload).Err()
}

func (q *RedisQueue) Consume(ctx context.Context, timeout time.Duration) (ConsumedJob, error) {
	if timeout <= 0 {
		timeout = defaultConsumeTimeout
	}

	payload, err := q.client.BRPopLPush(ctx, q.readyKey, q.processingKey, timeout).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ConsumedJob{}, ErrNoJob
		}

		return ConsumedJob{}, err
	}

	job, err := decodeJob(payload)
	if err != nil {
		return ConsumedJob{}, err
	}

	return ConsumedJob{Job: job, raw: payload}, nil
}

func (q *RedisQueue) Ack(ctx context.Context, consumed ConsumedJob) error {
	if consumed.raw == "" {
		return errors.New("consumed job payload is required")
	}

	return q.client.LRem(ctx, q.processingKey, 1, consumed.raw).Err()
}

func (q *RedisQueue) RetryLater(ctx context.Context, consumed ConsumedJob, runAt time.Time) error {
	if consumed.raw == "" {
		return errors.New("consumed job payload is required")
	}

	job := consumed.Job
	job.Attempt++
	payload, err := encodeJob(job)
	if err != nil {
		return err
	}

	pipe := q.client.TxPipeline()
	pipe.LRem(ctx, q.processingKey, 1, consumed.raw)
	pipe.ZAdd(ctx, q.delayedKey, redis.Z{
		Score:  float64(runAt.UnixMilli()),
		Member: payload,
	})
	_, err = pipe.Exec(ctx)
	return err
}

func (q *RedisQueue) DeadLetter(ctx context.Context, consumed ConsumedJob) error {
	if consumed.raw == "" {
		return errors.New("consumed job payload is required")
	}

	pipe := q.client.TxPipeline()
	pipe.LRem(ctx, q.processingKey, 1, consumed.raw)
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
		pipe.LPush(ctx, q.readyKey, payload)
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
