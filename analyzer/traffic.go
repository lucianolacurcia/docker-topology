package analyzer

import (
	"fmt"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/google/gopacket"
	_ "github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/lucianolacurcia/sprint-5/graphDB"
)

var (
	// edges is a set of idA+idB as key, if exists value, then the edge is already created in the graph
	noContainers map[string]bool
	edges        map[parIP]bool
	wg           sync.WaitGroup
)

type parIP struct {
	A, B string
}

func InitTrafficAnalizer() {
	edges = make(map[parIP]bool)
	noContainers = make(map[string]bool)
}

func MonitorAllContainers() {
	fmt.Println(containers)
	for _, container := range containersInfo {
		wg.Add(1)
		go MonitorPackets(container)
	}
	wg.Wait()
}

func MonitorPackets(containerA types.ContainerJSON) {
	if handle, err := pcap.OpenLive(containersVeth[containerA.ID], 256000, true, pcap.BlockForever); err != nil {
		panic(err)
	} else {
		ip, _ := GetContainerIPbyID(containerA.ID)
		// get only outgoing packets
		handle.SetBPFFilter("src " + ip)
		// handle.SetBPFFilter("src " + ip + " and (tcp[13] & 2 != 0)")
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		// process packets
		for packet := range packetSource.Packets() {
			if appL := packet.ApplicationLayer(); appL != nil {
				if appL := packet.ApplicationLayer().LayerContents(); string(appL) == "" {
					continue
				}
			}
			if linkL := packet.LinkLayer(); linkL != nil {
				fmt.Println(string(linkL.LinkFlow().String()))
			}
			if netL := packet.NetworkLayer(); netL != nil {
				fmt.Println(string(netL.NetworkFlow().String()))
				if edges[parIP{netL.NetworkFlow().Src().String(), netL.NetworkFlow().Dst().String()}] == false {
					edges[parIP{netL.NetworkFlow().Src().String(), netL.NetworkFlow().Dst().String()}] = true
					containerSrc, err := GetContainerByIP(netL.NetworkFlow().Src().String())
					if err != nil {
						panic(err)
					}
					containerDst, err := GetContainerByIP(netL.NetworkFlow().Dst().String())
					if err != nil {
						if _, miembro := noContainers[netL.NetworkFlow().Dst().String()]; !miembro {
							noContainers[netL.NetworkFlow().Dst().String()] = true
							err = graphDB.InsertNoContainerNode(netL.NetworkFlow().Dst().String())
							if err != nil {
								panic(err)
							}
						}
						if appL := packet.ApplicationLayer(); appL != nil {
							err = graphDB.AddDependencyNonContianerContainer(netL.NetworkFlow().Dst().String(), containerSrc, string(appL.LayerContents()))
						} else {
							err = graphDB.AddDependencyNonContianerContainer(netL.NetworkFlow().Dst().String(), containerSrc, "no app layer")
						}
						if err != nil {
							panic(err)
						}
						continue
					}
					if appL := packet.ApplicationLayer(); appL != nil {
						graphDB.AddDependency(containerSrc, containerDst, string(appL.LayerContents()))
					} else {
						err = graphDB.AddDependency(containerSrc, containerDst, "no app layer")
					}
					if err != nil {
						panic(err)
					}
				}
			}
			if appL := packet.ApplicationLayer(); appL != nil {
				fmt.Println(string(appL.LayerType().String()))
				fmt.Println(string(appL.LayerContents()))
				fmt.Println(string(appL.LayerPayload()))
			}
			if erroL := packet.ErrorLayer(); erroL != nil {
				fmt.Println(string(erroL.LayerType().String()))
				fmt.Println(string(erroL.LayerContents()))
				fmt.Println(string(erroL.LayerPayload()))
			}
		}
	}
	wg.Done()
}
