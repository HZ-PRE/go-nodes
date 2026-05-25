package utils

import (
	"log"

	"github.com/bwmarrin/snowflake"
)

var Node *snowflake.Node

func InitSnowflake(nodeID int64) {
	if nodeID < 0 || nodeID > 1023 {
		nodeID = 1
	}
	n, err := snowflake.NewNode(nodeID)
	if err != nil {
		log.Fatalf("failed to initialize snowflake node: %v", err)
	}
	Node = n
}
