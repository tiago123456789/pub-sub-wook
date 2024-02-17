package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
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

func GetQueueURL(sess *session.Session, queue string) (*sqs.GetQueueUrlOutput, error) {
	sqsClient := sqs.New(sess)

	result, err := sqsClient.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queue,
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func SendMessage(sess *session.Session, queueUrl string, messageBody string) error {
	sqsClient := sqs.New(sess)

	_, err := sqsClient.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    &queueUrl,
		MessageBody: aws.String(messageBody),
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

	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: "tiago",
		Config: aws.Config{
			Region:     aws.String("us-east-1"),
			HTTPClient: httpClient,
		},
	})

	if err != nil {
		fmt.Printf("Failed to initialize new session: %v", err)
		return
	}

	queueName := "new_request_dev"

	urlRes, err := GetQueueURL(sess, queueName)
	if err != nil {
		fmt.Printf("Got an error while trying to create queue: %v", err)
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

		err = SendMessage(sess, *urlRes.QueueUrl, string(dataJSON))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Interval server error",
			})
		}

		return c.Status(202).JSON(fiber.Map{})
	})

	app.Listen(":3000")
}
