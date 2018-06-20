package main

import (
	"bufio"
	"flag"
	"fmt"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/log"
	"harmony-benchmark/node"
	"harmony-benchmark/p2p"
	"math/rand"
	"os"
	"strings"
	"time"
	"harmony-benchmark/consensus"
	"encoding/hex"
	"strconv"
)

// Get numTxs number of Fake transactions based on the existing UtxoPool.
// The transactions are generated by going through the existing utxos and
// randomly select a subset of them as input to new transactions. The output
// address of the new transaction are randomly selected from 1 - 1000.
// NOTE: the genesis block should contain 1000 coinbase transactions adding
//       value to each address in [1 - 1000]. See node.AddMoreFakeTransactions()
func getNewFakeTransactions(dataNode *node.Node, numTxs int) []*blockchain.Transaction {

	/*
	     address - [
	                txId1 - [
	                        outputIndex1 - value1
	                        outputIndex2 - value2
	                       ]
	                txId2 - [
	                        outputIndex1 - value1
	                        outputIndex2 - value2
	                       ]
	               ]
	*/

	var outputs []*blockchain.Transaction
	count := 0
	countAll := 0

	for address, txMap := range dataNode.UtxoPool.UtxoMap {
		for txIdStr, utxoMap := range txMap {
			txId, err := hex.DecodeString(txIdStr)
			if err != nil {
				continue
			}
			for index, value := range utxoMap {
				countAll++
				if rand.Intn(100) <= 20 { // 20% sample rate to select UTXO to use for new transactions
				    // Spend the money of current UTXO to a random address in [1 - 1000]
					txin := blockchain.TXInput{txId, index, address}
					txout := blockchain.TXOutput{value, strconv.Itoa(rand.Intn(1000))}
					tx := blockchain.Transaction{nil, []blockchain.TXInput{txin}, []blockchain.TXOutput{txout}}
					tx.SetID()

					if count >= numTxs {
						continue
					}
					outputs = append(outputs, &tx)
					count++
				}
			}
		}
	}

	log.Debug("UTXO", "poolSize", countAll, "numTxsToSend", numTxs)
	return outputs
}

func getValidators(config string) []p2p.Peer {
	file, _ := os.Open(config)
	fscanner := bufio.NewScanner(file)
	var peerList []p2p.Peer
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		ip, port, status := p[0], p[1], p[2]
		if status == "leader" {
			continue
		}
		peer := p2p.Peer{Port: port, Ip: ip}
		peerList = append(peerList, peer)
	}
	return peerList
}

func getLeaders(config *[][]string) []p2p.Peer {
	var peerList []p2p.Peer
	for _, node := range *config {
		ip, port, status := node[0], node[1], node[2]
		if status == "leader" {
			peerList = append(peerList, p2p.Peer{Ip: ip, Port: port})
		}
	}
	return peerList
}

func readConfigFile(configFile string) [][]string {
	file, _ := os.Open(configFile)
	fscanner := bufio.NewScanner(file)

	result := [][]string{}
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		result = append(result, p)
	}
	return result
}

func main() {
	// Setup a stdout logger
	h := log.CallerFileHandler(log.StdoutHandler)
	log.Root().SetHandler(h)

	configFile := flag.String("config_file", "local_config.txt", "file containing all ip addresses and config")
	numTxsPerBatch := flag.Int("num_txs_per_batch", 1000, "number of transactions to send per message")
	flag.Parse()
	config := readConfigFile(*configFile)

	// Testing node
	dataNode := node.NewNode(&consensus.Consensus{})
	dataNode.AddMoreFakeTransactions()

	start := time.Now()
	totalTime := 60.0
	leaders := getLeaders(&config)
	time.Sleep(5 * time.Second) // wait for nodes to run
	for true {
		t := time.Now()
		if t.Sub(start).Seconds() >= totalTime {
			fmt.Println(int(t.Sub(start)), start, totalTime)
			break
		}
		txsToSend := getNewFakeTransactions(&dataNode, *numTxsPerBatch)
		msg := node.ConstructTransactionListMessage(txsToSend)
		log.Debug("[Generator] Sending txs...", "numTxs", len(txsToSend))
		p2p.BroadcastMessage(leaders, msg)

		dataNode.UtxoPool.Update(txsToSend)
		time.Sleep(1 * time.Second) // 10 transactions per second
	}
	msg := node.ConstructStopMessage()
	peers := append(getValidators(*configFile), leaders...)
	p2p.BroadcastMessage(peers, msg)
}
