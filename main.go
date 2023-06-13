package main

import (
	"log"
	"os"

	"r3w3/core"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	app, err := core.NewApp(core.AppConfigOpts{
		JsonRPCWebsocket: "ws://127.0.0.1:10001/ws",
		PrivateKeyString: os.Getenv("PRIVATE_KEY"),
	})
	if err != nil {
		log.Fatalln("Could not create new app: ", err.Error())
	}

	if err = app.SetNewValuesAtTimeInterval([]int64{101, 222, 1003, 105, 3345, 883, 992}); err != nil {
		log.Fatalln("Could not run SetNewValuesAtTimeInterval: ", err.Error())
	}
}
