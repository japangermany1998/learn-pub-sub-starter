package main

import (
	"fmt"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
	"os"
)

func main() {
	fmt.Println("Starting Peril server...")

	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func(conn *amqp.Connection) {
		err := conn.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("Connection closed")
	}(conn)
	fmt.Println("Connect successfully")

	channel, err := conn.Channel()
	if err != nil {
		fmt.Println(err)
		return
	}
	//err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
	//	IsPaused: true,
	//})
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	err = pubsub.SubscribeGob(
		conn, routing.ExchangePerilTopic, routing.GameLogSlug, routing.GameLogSlug+".*", 0, handleLog())
	if err != nil {
		fmt.Println(err)
		return
	}
	gamelogic.PrintServerHelp()
	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		switch words[0] {
		case routing.PauseKey:
			fmt.Println("Sending a pause message")
			err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
				IsPaused: true,
			})
			break
		case "resume":
			fmt.Println("Sending a resume message")
			err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
				IsPaused: false,
			})
			break
		case "quit":
			fmt.Println("Exiting")
			os.Exit(0)
			break
		default:
			fmt.Println("I don't understand the command")
		}
	}
	//done := make(chan os.Signal, 1)
	//signal.Notify(done, os.Interrupt)
	//<-done
	//fmt.Println("Shut down program")

}

func handleLog() func(routing.GameLog) string {
	return func(log routing.GameLog) string {
		defer fmt.Print(">")

		err := gamelogic.WriteLog(log)
		if err != nil {
			return "NackRequeue"
		}
		return "Ack"
	}
}
