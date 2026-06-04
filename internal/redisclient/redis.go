package redisclient

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}

func Keys(saleID string) struct {
	QueueSeq, NextAdmit, AdmitRate string
} {
	prefix := "sale:" + saleID
	return struct {
		QueueSeq, NextAdmit, AdmitRate string
	}{
		QueueSeq:  prefix + ":queue_seq",
		NextAdmit: prefix + ":next_admit",
		AdmitRate: prefix + ":admit_tokens",
	}
}
