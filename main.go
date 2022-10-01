package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lucianolacurcia/sprint-5/analyzer"
	"github.com/lucianolacurcia/sprint-5/graphDB"
)

func main() {
	// listen to os signals:
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)
		done <- true
	}()

	// TODO: parametrizar
	graphDB.InitDB("neo4j://localhost:7687", "neo4j", "s3cr3t")

	analyzer.InitDockerAnalyzer()
	analyzer.InitTrafficAnalizer()

	go analyzer.ListenEvents()

	go analyzer.MonitorAllContainers()

	fmt.Println("awaiting signal")
	<-done
	fmt.Println("terminating...")
	// TODO: eliminar nodos, apagar monitores
	graphDB.DropDB()
	fmt.Println("exiting")

}
