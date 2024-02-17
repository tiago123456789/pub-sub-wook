package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/gofiber/fiber/v2"
)

type URLSubscribed struct {
	Headers map[string]string      `json:"headers"`
	Method  string                 `json:"method"`
	Data    map[string]interface{} `json:"data"`
	Url     string                 `json:"url"`
}

func notifySubscribes(urlSubscribed []URLSubscribed) {
	for _, item := range urlSubscribed {

		jsonBody, _ := json.Marshal(item.Data)
		bodyReader := bytes.NewReader(jsonBody)

		req, _ := http.NewRequest(item.Method, item.Url, bodyReader)

		for key, value := range item.Headers {
			req.Header.Set(key, value)
		}
		req.Header.Set("Content-Type", "application/json")

		client := http.Client{
			Timeout: 5 * time.Second,
		}

		client.Do(req)
	}

}

func SendMessage(client *sqs.Client, queueUrl string, messageBody string) error {
	_, err := client.SendMessage(context.TODO(), &sqs.SendMessageInput{
		MessageBody: &messageBody,
		QueueUrl:    aws.String(queueUrl),
	})

	return err
}

func main() {
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithSharedConfigProfile("tiago"),
		config.WithHTTPClient(httpClient), // Use your custom HTTP client here
	)
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}

	// Create a new SQS client
	client := sqs.NewFromConfig(cfg)

	// Specify the URL of the SQS queue
	queueURL := "https://sqs.us-east-1.amazonaws.com/507403822990/new_request_dev"

	if err != nil {
		fmt.Printf("Failed to initialize new session: %v", err)
		return
	}

	app := fiber.New()

	type Payload struct {
		Event string                 `json:"event"`
		Token string                 `json:"token"`
		Data  map[string]interface{} `json:"data"`
	}

	app.Post("/:token", func(c *fiber.Ctx) error {
		var payload Payload

		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"errors": err.Error(),
			})
		}

		payload.Token = c.Params("token")
		if len(payload.Event) == 0 {
			payload.Event = c.Query("event")
		}

		dataJSON, _ := json.Marshal(payload)

		err = SendMessage(client, queueURL, string(dataJSON))
		if err != nil {
			log.Fatal(err)
			return c.Status(500).JSON(fiber.Map{
				"message": "Interval server error",
			})
		}

		return c.SendStatus(202)
	})

	app.Listen(":3000")
}
