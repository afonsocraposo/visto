package main

import (
	"fmt"
	push "github.com/SherClockHolmes/webpush-go"
	"log"
)

func main() {
	private, public, err := push.GenerateVAPIDKeys()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("VISTO_WEB_PUSH_PUBLIC_KEY=%s\nVISTO_WEB_PUSH_PRIVATE_KEY=%s\n", public, private)
}
