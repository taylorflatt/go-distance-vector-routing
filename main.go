package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var netMap = &globalNetworkMap{
	lock:    sync.RWMutex{},
	routers: make(map[string]chan map[string]*neighbor),
}

type globalNetworkMap struct {
	lock    sync.RWMutex
	routers map[string]chan map[string]*neighbor
}

type router struct {
	id    string
	lock  sync.RWMutex
	log   string
	ch    chan map[string]*neighbor
	table map[string]*neighbor
}

type neighbor struct {
	name string
	cost int32
}

func createRouter(id string, neighbors []neighbor) {

	log.Println("Entered createRouter")

	// Construct an empty routing table for the router.
	rt := make(map[string]*neighbor)

	// Add itself to the routing table first.
	rt[id] = &neighbor{
		name: id,
		cost: 0,
	}

	// Add the router's initial neighbors.
	for _, n := range neighbors {
		rt[n.name] = &neighbor{
			name: n.name,
			cost: n.cost,
		}
	}

	// Create the router.
	r := &router{
		id:    id,
		lock:  sync.RWMutex{},
		ch:    make(chan map[string]*neighbor, 100),
		table: rt,
	}

	netMap.lock.Lock()
	defer netMap.lock.Unlock()

	// Add the router to the global list.
	netMap.routers[id] = r.ch

	fmt.Println(netMap.routers)

	// Run the router threads.
	go routerThread(*r)
}

func (ot *router) UpdateTable(nt map[string]*neighbor) {

	ot.lock.Lock()
	defer ot.lock.Unlock()
	var updated = false
	var nID string

	// We have 2 table update cases:
	// 		1) Add a new value to our table we didn't already have.
	// 		2) Update a value with a faster path (change the hop).

	for val, tNextCost := range nt {
		if val == tNextCost.name && tNextCost.cost == 0 {
			nID = val
		}
	}

	for destination, nextCost := range nt {
		_, ok := ot.table[destination]
		cost := nextCost.cost + nt[ot.id].cost
		if ok {
			// If our cost is larger than the incoming cost, update our table.
			if ot.table[destination].cost > cost {
				ot.table[destination].cost = cost
				ot.table[destination].name = nextCost.name
				updated = true
			}
		} else {
			// It doesn't exist, add it to our table.
			ot.table[destination] = &neighbor{
				name: nID,
				cost: cost,
			}
			updated = true
		}
	}

	// If the table updated, send the updated table to its neighbors.
	if updated {
		ot.sendToNeighbors()
	}
}

func (r *router) tableInfo() {

	if r.id == "B" {
		log.Println("		" + r.id + "		")
		log.Println("Destination | Next | Cost")
		for id, n := range r.table {
			log.Println("     " + id + "      " + "  " + n.name + "   " + "  " + strconv.Itoa(int(n.cost)) + "   ")
		}
		log.Println("")
	}
}

func (r *router) sendToNeighbors() {

	// Need to wait for other routers to begin listening on their channels
	// prior to sending the updated tables.
	log.Println("Inside sendToNeighbors")
	time.Sleep(25 * time.Millisecond)

	for id, n1 := range r.table {
		if id == n1.name && id != r.id {
			fmt.Println(r.id + " is sending his table to : ")
			fmt.Println(netMap.routers[id])
			netMap.routers[id] <- r.table
		}
	}
}

func routerThread(r router) {

	log.Println("INITIAL TABLE INFORMATION")
	r.tableInfo()

	// Send my table to all my neighbors.
	go r.sendToNeighbors()

	fmt.Println(r.id + "'s table is ")
	fmt.Println(r.ch)

	for {
		select {
		case update := <-r.ch:
			log.Println("I see the update.")
			r.UpdateTable(update)
			r.tableInfo()
		}
	}

}

func main() {

	var neighbors []neighbor
	var neighbors2 []neighbor
	var neighbors3 []neighbor

	neighbors = append(neighbors, neighbor{
		name: "B",
		cost: 5,
	})

	neighbors = append(neighbors, neighbor{
		name: "C",
		cost: 12,
	})

	neighbors2 = append(neighbors2, neighbor{
		name: "A",
		cost: 5,
	})

	neighbors3 = append(neighbors3, neighbor{
		name: "A",
		cost: 12,
	})

	go createRouter("A", neighbors)
	go createRouter("B", neighbors2)
	go createRouter("C", neighbors3)

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	<-ch

}
