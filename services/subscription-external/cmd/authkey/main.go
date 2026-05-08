package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"

	"github.com/seidu626/subscription-manager/subscription-external/internal/utils"
)

func main() {
	partnerID := flag.String("partner", "", "partner service id")
	psk := flag.String("psk", "", "preshared key (16/24/32 bytes)")
	ts := flag.String("ts", "", "timestamp in milliseconds (optional)")
	flag.Parse()

	if *partnerID == "" || *psk == "" {
		log.Fatal("missing required flags: --partner and --psk")
	}

	if *ts == "" {
		key, err := utils.GenerateAuthKeyV2(*partnerID, *psk)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(key)
		return
	}

	tsMs, err := strconv.ParseInt(*ts, 10, 64)
	if err != nil {
		log.Fatalf("invalid ts: %v", err)
	}
	key, err := utils.GenerateAuthKeyWithTimestamp(*partnerID, *psk, tsMs)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(key)
}
