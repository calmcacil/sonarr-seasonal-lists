package model

import (
	"fmt"
	"strconv"
	"strings"
)

type SeasonKey struct {
	Season string
	Year   int
}

func ParseSeasonKey(key string) (SeasonKey, error) {
	parts := strings.SplitN(key, "-", 2)
	if len(parts) != 2 {
		return SeasonKey{}, fmt.Errorf("invalid season key: %s", key)
	}
	if parts[0] == "" {
		return SeasonKey{}, fmt.Errorf("invalid season key: %s", key)
	}
	y, err := strconv.Atoi(parts[1])
	if err != nil {
		return SeasonKey{}, fmt.Errorf("invalid year in season key: %s: %w", key, err)
	}
	return SeasonKey{Season: parts[0], Year: y}, nil
}

func (k SeasonKey) String() string {
	return fmt.Sprintf("%s-%d", k.Season, k.Year)
}
