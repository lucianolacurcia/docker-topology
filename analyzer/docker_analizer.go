package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/lucianolacurcia/sprint-5/graphDB"
)

var (
	containersIP   map[string]string
	containersVeth map[string]string
	containers     map[string]types.Container
	containersInfo map[string]types.ContainerJSON
	dockerNetworks map[string]types.NetworkResource
	estaEnDB       map[string]bool
)

func InitDockerAnalyzer() {
	estaEnDB = make(map[string]bool)
	containers = make(map[string]types.Container)
	containersVeth = make(map[string]string)
	containersInfo = make(map[string]types.ContainerJSON)
	dockerNetworks = make(map[string]types.NetworkResource)
	containersIP = make(map[string]string)

	fetchContainers()
	fetchContainersInfo()
	fetchNetworks()
	fetchContainersIp()
	fetchContainersVeth()
	addContainersToDB()
}

func fetchContainers() {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	containersAux, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, container := range containersAux {
		containers[container.ID] = container
	}
}

func fetchContainersInfo() {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	for k := range containers {
		containerJSON, err := cli.ContainerInspect(context.Background(), k)
		if err != nil {
			panic(err)
		}
		containersInfo[k] = containerJSON
	}
}

func fetchContainersVeth() {
	out, err := exec.Command("dockervethmin").Output()
	if err != nil {
		log.Fatal(err)
	}
	tabla := string(out)
	campos := strings.Fields(tabla)
	containersVethAux := make(map[string]string)
	for i := 0; i < len(campos); i = i + 2 {
		containersVethAux[campos[i]] = campos[i+1]
	}
	containersVeth = containersVethAux
}

func fetchNetworks() {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	dockerNetworksAux, err := cli.NetworkList(context.Background(), types.NetworkListOptions{})
	if err != nil {
		panic(err)
	}

	for _, net := range dockerNetworksAux {
		network, err := cli.NetworkInspect(context.Background(), net.ID, types.NetworkInspectOptions{Verbose: true})
		if err != nil {
			panic(err)
		}

		dockerNetworks[network.ID] = network
	}
}

func fetchNetworkByID(id string) {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	network, err := cli.NetworkInspect(context.Background(), id, types.NetworkInspectOptions{Verbose: true})
	if err != nil {
		panic(err)
	}

	dockerNetworks[network.ID] = network
}

func fetchContainersIp() {
	containersIPAux := make(map[string]string)
	for _, network := range dockerNetworks {
		for idContainer, endpoints := range network.Containers {
			ip := endpoints.IPv4Address
			ip = strings.Split(ip, "/")[0]
			containersIPAux[idContainer] = ip
		}
	}
	containersIP = containersIPAux
}

func fetchContainerById(id string) error {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	containersAux, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	for _, container := range containersAux {
		if container.ID == id {
			containers[container.ID] = container
			return nil
		}
	}
	return errors.New("Container not found")
}

func fetchContainerInfoById(id string) error {
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	containerJSON, err := cli.ContainerInspect(context.Background(), id)
	if err != nil {
		return err
	}
	containersInfo[id] = containerJSON
	return nil
}

func fetchContainerVethById(id string) error {
	out, err := exec.Command("dockervethmin").Output()
	if err != nil {
		panic(err)
	}
	tabla := string(out)
	campos := strings.Fields(tabla)
	for i := 0; i < len(campos); i = i + 2 {
		if campos[i] == id {
			containersVeth[campos[i]] = campos[i+1]
			return nil
		}
	}
	return errors.New("no veth associated with container id provided.")
}

func addContainersToDB() {
	for _, container := range containersInfo {

		ip, _ := GetContainerIPbyID(container.ID)
		err := graphDB.InsertContainer(container, ip)
		if err != nil {
			panic(err)
		}
	}
}

func GetContainerByIP(ip string) (types.ContainerJSON, error) {
	for id, ipAux := range containersIP {
		if ipAux == ip {
			return containersInfo[id], nil
		}
	}
	return types.ContainerJSON{}, errors.New("Container with ip provided not found")
}

func GetContainerIPbyID(id string) (string, error) {
	if ip, member := containersIP[id]; member {
		return ip, nil
	}
	return "", errors.New("no ip stored for that id")
}

func newContainerCreated(id string) error {
	err := fetchContainerById(id)
	if err != nil {
		panic(err)
	}
	err = fetchContainerInfoById(id)
	if err != nil {
		panic(err)
	}
	ip, _ := GetContainerIPbyID(id)

	err = graphDB.InsertContainer(containersInfo[id], ip)
	if err != nil {
		panic(err)
	}
	estaEnDB[id] = true
	return err
}

func containerStarted(id string) error {
	err := fetchContainerById(id)
	if err != nil {
		panic(err)
	}

	err = fetchContainerInfoById(id)
	if err != nil {
		panic(err)
	}

	ip, _ := GetContainerIPbyID(id)
	if !estaEnDB[id] {
		err = graphDB.InsertContainer(containersInfo[id], ip)
		if err != nil {
			panic(err)
		}
		estaEnDB[id] = true
	}

	err = graphDB.UpdateContainer(containersInfo[id], ip)
	return err
}

func containerStopped(id string) error {
	err := fetchContainerById(id)
	if err != nil {
		panic(err)
	}
	err = fetchContainerInfoById(id)
	if err != nil {
		panic(err)
	}
	ip, _ := GetContainerIPbyID(id)
	err = graphDB.UpdateContainer(containersInfo[id], ip)
	return err
}

func containerDestroyed(id string) error {
	delete(containersIP, id)
	delete(containers, id)
	delete(containersInfo, id)
	delete(containersVeth, id)
	err := graphDB.DeleteContainer(id)
	if err != nil {
		panic(err)
	}
	delete(estaEnDB, id)
	return err
}

func networkCreated(id string) error {
	fetchNetworkByID(id)
	return nil
}

func networkDestroyed(id string) {
	delete(dockerNetworks, id)
}

func networkConnected(idNetwork, idContainer string) {
	// update network
	fetchNetworkByID(idNetwork)

	// update container
	err := fetchContainerById(idContainer)
	if err != nil {
		panic(err)
	}
	err = fetchContainerInfoById(idContainer)
	if err != nil {
		panic(err)
	}

	// update veth and ip
	err = fetchContainerVethById(idContainer)
	if err != nil {
		panic(err)
	}
	fetchContainersIp()
	ip, _ := GetContainerIPbyID(idContainer)
	graphDB.UpdateContainer(containersInfo[idContainer], ip)

	go MonitorPackets(containersInfo[idContainer])

}

func networkDisconnect(idNetwork, idContainer string) {
	// update network
	fetchNetworkByID(idNetwork)

	// update container
	err := fetchContainerById(idContainer)
	if err != nil {
		panic(err)
	}
	err = fetchContainerInfoById(idContainer)
	if err != nil {
		panic(err)
	}
}

func ListenEvents() {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options := types.EventsOptions{}

	eventsChan, errChan := cli.Events(ctx, options)

	go func() {
		for event := range eventsChan {
			if event.Type == "network" && event.Action == "create" {
				fmt.Printf("Network created: %s\n", event.Actor.ID)
				err := networkCreated(event.Actor.ID)
				if err != nil {
					panic(err)
				}
			} else if event.Type == "container" && event.Action == "create" {
				fmt.Printf("Container created: %s\n", event.Actor.ID)
				err := newContainerCreated(event.Actor.ID)
				if err != nil {
					panic(err)
				}
			} else if event.Type == "network" && event.Action == "connect" {
				fmt.Printf("Network connected: %s\n", event.Actor.ID)
				empJSON, err := json.MarshalIndent(event, "", "  ")
				networkConnected(event.Actor.ID, event.Actor.Attributes["container"])
				if err != nil {
					log.Fatalf(err.Error())
				}
				fmt.Println(event.Actor.Attributes["container"])
				fmt.Printf("%s\n", string(empJSON))
			} else if event.Type == "container" && event.Action == "start" {
				fmt.Printf("Container started: %s\n", event.Actor.ID)
				err := containerStarted(event.Actor.ID)
				if err != nil {
					panic(err)
				}
			} else if event.Type == "network" && event.Action == "disconnect" {
				fmt.Printf("network destroyed: %s\n", event.Actor.ID)
				networkDisconnect(event.Actor.ID, event.Actor.Attributes["container"])
				if err != nil {
					panic(err)
				}

			} else if event.Type == "container" && event.Action == "stop" {
				fmt.Printf("Container stopped: %s\n", event.Actor.ID)
				err := containerStopped(event.Actor.ID)
				if err != nil {
					panic(err)
				}

			} else if event.Type == "container" && event.Action == "destroy" {
				fmt.Printf("Container destroyed: %s\n", event.Actor.ID)
				err := containerDestroyed(event.Actor.ID)
				if err != nil {
					panic(err)
				}

			} else if event.Type == "network" && event.Action == "destroy" {
				fmt.Printf("network destroyed: %s\n", event.Actor.ID)
				networkDestroyed(event.Actor.ID)
				if err != nil {
					panic(err)
				}
			}

		}
	}()

	if err := <-errChan; err != nil {
		panic(err)
	}
}
