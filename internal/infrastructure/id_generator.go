package infrastructure

import "fmt"
import "time"

type IDGenerator interface {
	GenerateID() string
}

type TimestampIDGenerator struct{}

func (t TimestampIDGenerator) GenerateID() string {
	return fmt.Sprintf("art_%d", time.Now().UnixNano())
}
