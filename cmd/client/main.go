package main

import (
	"fmt"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"os"
)

func main() {
	fmt.Println("Starting Peril client...")

	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		fmt.Println(err)
		return
	}
	publishCh, err := conn.Channel()
	if err != nil {
		log.Fatalf("could not create channel: %v", err)
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

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println(err)
		return
	}

	gamestate := gamelogic.NewGameState(username)

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect,
		"pause."+username,
		routing.PauseKey,
		1,
		handlerPause(gamestate))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"."+username,
		routing.ArmyMovesPrefix+".*",
		1,
		handlerMove(publishCh, gamestate))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		routing.WarRecognitionsPrefix+".*",
		0,
		handlerWar(gamestate))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for {
		words := gamelogic.GetInput()
		switch words[0] {
		case "spawn":
			if err = gamestate.CommandSpawn(words); err != nil {
				fmt.Println(err)
				break
			}
			break
		case "move":
			armymove, err := gamestate.CommandMove(words)
			if err != nil {
				fmt.Println(err)
				break
			}
			ch, err := conn.Channel()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			err = pubsub.PublishJSON(ch, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+armymove.Player.Username, armymove)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			fmt.Println("Move successfully")
			break
		case "status":
			gamestate.CommandStatus()
			break
		case "help":
			gamelogic.PrintClientHelp()
			break
		case "spam":
			fmt.Println("Spamming not allowed yet!")
			break
		case "quit":
			gamelogic.PrintQuit()
			break
		default:
			fmt.Println("I don't understand the command")
		}
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) string {
	return func(ps routing.PlayingState) string {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return "Ack"
	}
}

func handlerMove(ch *amqp.Channel, gs *gamelogic.GameState) func(gamelogic.ArmyMove) string {
	return func(am gamelogic.ArmyMove) string {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(am)
		fmt.Println(outcome)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return "Ack"
		case gamelogic.MoveOutcomeSamePlayer:
			return "Ack"
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(ch, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix+"."+gs.GetUsername(),
				gamelogic.RecognitionOfWar{
					Attacker: am.Player,
					Defender: gs.GetPlayerSnap(),
				})
			if err != nil {
				fmt.Printf("error: %s\n", err)
				return "NAckRequeue"
			}
			return "NAckRequeue"
		}
		return "NackDiscard"
	}
}

func handlerWar(gs *gamelogic.GameState) func(am gamelogic.RecognitionOfWar) string {
	return func(rw gamelogic.RecognitionOfWar) string {
		defer fmt.Print("> ")
		outcome, _, _ := gs.HandleWar(rw)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return "NackRequeue"
		case gamelogic.WarOutcomeNoUnits:
			return "NackDiscard"
		case gamelogic.WarOutcomeOpponentWon:
			return "ack"
		case gamelogic.WarOutcomeYouWon:
			return "ack"
		case gamelogic.WarOutcomeDraw:
			return "ack"
		}
		fmt.Println("error")
		return "NackDiscard"
	}
}
