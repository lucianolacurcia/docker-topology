// Package graphDB provides handling for neo4j needed operations
package graphDB

import (
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
)

var (
	driver neo4j.Driver
)

func InitDB(url string, user string, pass string) error {
	var err error
	driver, err = neo4j.NewDriver(url, neo4j.BasicAuth(user, pass, ""))
	if err != nil {
		panic(err)
	}
	return nil
}

func InsertContainer(container types.ContainerJSON, ip string) error {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	ports := ""
	for portContainer, portHostMap := range container.NetworkSettings.Ports {
		for _, portHost := range portHostMap {
			ports = portContainer.Port() + "/" + portContainer.Proto() + " -> " + portHost.HostIP + ":" + portHost.HostPort + "\n"
		}
	}
	_, err = session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		result, err := transaction.Run(
			"CREATE (a:Container :"+container.State.Status+" {name: $name, id: $id, ports: $ports, ip: $ip, hostname: $hostname})",
			map[string]interface{}{"name": container.Name, "id": container.ID, "ports": ports, "pid": container.State.Pid, "ip": ip, "hostname": hostname})
		if err != nil {
			return nil, err
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, result.Err()
	})
	if err != nil {
		return err
	}
	return err
}

func DropDB() error {
	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		result, err := transaction.Run("MATCH (n) DETACH DELETE (n)", map[string]interface{}{})
		if err != nil {
			return nil, err
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, result.Err()
	})
	if err != nil {
		return err
	}
	return err
}

func InsertNoContainerNode(ip string) error {
	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		result, err := transaction.Run(
			"CREATE (a:NoContainer{ip: $ip})",
			map[string]interface{}{"ip": ip})
		if err != nil {
			return nil, err
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, result.Err()
	})
	if err != nil {
		return err
	}
	return err
}

func DeleteContainer(id string) error {
	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		result, err := transaction.Run(
			"MATCH (n {id: $id}) DETACH DELETE n",
			map[string]interface{}{"id": id})
		if err != nil {
			return nil, err
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, result.Err()
	})
	if err != nil {
		return err
	}
	return err
}

func UpdateContainer(container types.ContainerJSON, ip string) error {
	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	// get labels of node for deleting
	labels, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		result, err := transaction.Run(
			"MATCH (n {id: $id}) return labels(n) as labels",
			map[string]interface{}{"id": container.ID})
		if err != nil {
			return nil, err
		}

		var labels []string
		for result.Next() {
			record := result.Record()

			labs, ok := record.Get("labels")
			if !ok {
				return nil, fmt.Errorf("Couldn't get labels")
			}

			var l []string
			if labs != nil {
				l, err = parseInterfaceToString(labs)
				if err != nil {
					return nil, err
				}
			}

			labels = l
		}
		return labels, result.Err()
	})
	if err != nil {
		return err
	}

	assertedLabels, ok := labels.([]string)
	if !ok {
		return nil
	}

	// ports to string
	ports := ""
	for portContainer, portHostMap := range container.NetworkSettings.Ports {
		for _, portHost := range portHostMap {
			ports = portContainer.Port() + "/" + portContainer.Proto() + " -> " + portHost.HostIP + ":" + portHost.HostPort + "\n"
		}
	}

	//labels string
	labelsToRemove := ""
	for _, label := range assertedLabels {
		labelsToRemove += ":" + label
	}

	_, err = session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		result, err := transaction.Run(
			"MATCH (n:Container {id: $id}) "+
				" REMOVE n"+labelsToRemove+
				" SET n:Container:"+container.State.Status+
				" SET n.name = $name"+
				" SET n.ports = $ports"+
				" SET n.id = $id"+
				" SET n.ip = $ip",
			map[string]interface{}{"name": container.Name, "id": container.ID, "ports": ports, "ip": ip})
		if err != nil {
			return nil, err
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, result.Err()
	})
	if err != nil {
		return err
	}

	return err
}

func parseInterfaceToString(i interface{}) (s []string, err error) {
	converted, ok := i.([]interface{})
	if !ok {
		return nil, fmt.Errorf("Argument is not a slice")
	}
	for _, v := range converted {
		s = append(s, v.(string))
	}
	return
}

func AddDependency(contOri, contDest types.ContainerJSON, appLayerHeader string) error {
	fmt.Printf("Añadiendo flecha: %s -> %s\n", contOri.Name, contDest.Name)
	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		result, err := transaction.Run("MATCH (a:Container), (b:Container) WHERE a.id = $idA AND b.id = $idB CREATE (a)-[r:DEPENDE_DE {AppLayerContent: $appLayerHeader}]->(b) RETURN type(r), r.name", map[string]interface{}{"idA": contOri.ID, "idB": contDest.ID, "appLayerHeader": appLayerHeader})
		if err != nil {
			return nil, err
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, result.Err()
	})
	if err != nil {
		panic(err)
	}
	return err
}

func AddDependencyNonContianerContainer(ip string, contOri types.ContainerJSON, appLayerHeader string) error {
	fmt.Printf("Añadiendo flecha: %s -> %s\n", contOri.Name, ip)
	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		result, err := transaction.Run("MATCH (a:Container), (b:NoContainer) WHERE a.id = $idA AND b.ip = $ip CREATE (a)-[r:DEPENDE_DE {AppLayerContent: $appLayerHeader}]->(b) RETURN type(r), r.name", map[string]interface{}{"idA": contOri.ID, "ip": ip, "appLayerHeader": appLayerHeader})
		if err != nil {
			return nil, err
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, result.Err()
	})
	if err != nil {
		panic(err)
	}
	return err
}
