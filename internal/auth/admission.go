package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func AdmissionUsedKey(saleID, jti string) string {
	return fmt.Sprintf("admission:used:%s:%s", saleID, jti)
}

// ConsumeAdmission marks a JWT as used (single-use). Returns false if already consumed.
func ConsumeAdmission(ctx context.Context, rdb *redis.Client, saleID, jti string, ttl time.Duration) (bool, error) {
	key := AdmissionUsedKey(saleID, jti)
	ok, err := rdb.SetNX(ctx, key+":consumed", "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}
