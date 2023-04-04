package main

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"log"
	"time"
)

var kafkaBrokerAddresses = []string{
	"localhost:29092",
	"localhost:39092",
}

func main() {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   kafkaBrokerAddresses,
		Topic:     "messages",
		Partition: 0,
	})

	log.Println("start reading kafka")
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)

		m, err := r.ReadMessage(ctx)
		if err != nil {
			log.Println(err)
			break
		}
		cancel()
		fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))
	}

	if err := r.Close(); err != nil {
		log.Fatal("failed to close reader:", err)
	}
}
