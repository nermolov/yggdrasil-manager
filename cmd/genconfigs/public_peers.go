package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"golang.org/x/net/html"
)

const PEER_INDEX_URL = "https://publicpeers.neilalexander.dev/"
const PEER_COUNTRY = "united-states"
const PEER_PUBLIC_COUNT = 3

func hasAttribute(node *html.Node, name, value string) bool {
	if node.Type != html.ElementNode {
		return false
	}
	for _, attr := range node.Attr {
		if attr.Key == name && attr.Val == value {
			return true
		}
	}
	return false
}

// fetchOnlinePeers fetches the list of online peers from PEER_INDEX_URL for the specified PEER_COUNTRY
func fetchOnlinePeers() []string {
	peerIndex, err := http.Get(PEER_INDEX_URL)
	if err != nil {
		panic(err)
	}
	if peerIndex.StatusCode != http.StatusOK {
		panic(fmt.Sprintf("failed to fetch peer index: %s", peerIndex.Status))
	}
	defer peerIndex.Body.Close()

	doc, err := html.Parse(peerIndex.Body)
	if err != nil {
		panic(err)
	}

	peers := []string{}

	inCountry := false
	for node := range doc.Descendants() {
		// find table header
		if found := hasAttribute(node, "id", "country"); found {
			if node.FirstChild.Data == PEER_COUNTRY {
				inCountry = true
			} else {
				inCountry = false
			}
			continue
		}
		// collect online peers in the country section
		if inCountry {
			if found := hasAttribute(node, "class", "statusgood"); found {
				for subnode := range node.Descendants() {
					if found := hasAttribute(subnode, "id", "address"); found {
						peers = append(peers, subnode.FirstChild.Data)
					}
				}
			}
		}
	}

	return peers
}

type peerStat struct {
	Address string
	Latency time.Duration
}
type peerStats []peerStat

func (ps peerStats) Len() int           { return len(ps) }
func (ps peerStats) Swap(i, j int)      { ps[i], ps[j] = ps[j], ps[i] }
func (ps peerStats) Less(i, j int) bool { return ps[i].Latency < ps[j].Latency }

func isConfiguredPeerProtocol(address string) bool {
	return strings.HasPrefix(address, fmt.Sprintf("%v://", PROTOCOL))
}

// selectPublicPeers selects the top PEER_PUBLIC_COUNT online public peers based on latency.
// Ignores any peers that do not support the PROTOCOL or have packet loss.
func selectPublicPeers() []string {
	fmt.Println("Selecting public peers")

	onlinePeers := fetchOnlinePeers()

	peerStatsList := peerStats{}

	for _, peer := range onlinePeers {
		if !isConfiguredPeerProtocol(peer) {
			continue
		}

		peerUrl, err := url.Parse(peer)
		if err != nil {
			fmt.Printf("Failed to parse peer URL %s: %v\n", peer, err)
			continue
		}

		pinger, err := probing.NewPinger(peerUrl.Hostname())
		if err != nil {
			fmt.Printf("Failed to create pinger for peer %s: %v\n", peer, err)
			continue
		}

		pinger.Count = 5
		pinger.Timeout = 500 * time.Millisecond

		err = pinger.Run()
		if err != nil {
			fmt.Printf("Could not run pinger for peer %s: %v\n", peer, err)
			continue
		}

		stats := pinger.Statistics()
		if stats.PacketLoss > 0 {
			fmt.Printf("Skipping peer %s, has packet loss: %.2f%%\n", peer, stats.PacketLoss)
			continue
		}

		peerStatsList = append(peerStatsList, peerStat{
			Address: peer,
			Latency: stats.AvgRtt,
		})
	}

	sort.Sort(peerStatsList)
	peerStatsList = peerStatsList[:PEER_PUBLIC_COUNT]

	fmt.Println("Selected public peers:")
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for _, ps := range peerStatsList {
		fmt.Fprintf(w, "%v\t%v\n", ps.Latency, ps.Address)
	}
	w.Flush()

	selectedPeers := []string{}
	for _, ps := range peerStatsList {
		selectedPeers = append(selectedPeers, ps.Address)
	}

	return selectedPeers
}
