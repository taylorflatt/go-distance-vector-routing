package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var watchTable string

var netMap = &globalNetworkMap{
	lock:    sync.RWMutex{},
	ch:      make(map[string]chan routerTable),
	routers: make(map[string]*router),
}

type routerTable struct {
	id    string
	table map[string]*neighbor
}

type globalNetworkMap struct {
	lock    sync.RWMutex
	ch      map[string]chan routerTable
	routers map[string]*router
}

type router struct {
	id    string
	lock  sync.RWMutex
	log   string
	ch    chan routerTable
	table map[string]*neighbor
}

type neighbor struct {
	name string
	cost int32
}

func createRouter(id string, neighbors []neighbor) {

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
		ch:    make(chan routerTable, 100),
		table: rt,
	}

	//netMap.lock.Lock()
	//defer netMap.lock.Unlock()

	// Add the router to the global list.
	netMap.ch[id] = r.ch

	netMap.routers[id] = r

	//fmt.Println(id)
	//fmt.Println(netMap.routers[id].ch)

	// Run the router threads.
	go routerThread(*r)
}

func (ot *router) UpdateTable(nt routerTable) {

	// Need lock here (was netmap lock)

	//netMap.routers[nt.id].lock.Lock()
	//defer netMap.routers[nt.id].lock.Unlock()
	//netMap.lock.Lock()
	//defer netMap.lock.Lock()

	ot.lock.Lock()
	defer ot.lock.Unlock()
	var updated = false
	var nID string

	// We have 2 table update cases:
	// 		1) Add a new value to our table we didn't already have.
	// 		2) Update a value with a faster path (change the hop).

	for val, tNextCost := range nt.table {
		if val == tNextCost.name && tNextCost.cost == 0 {
			nID = val
		}
	}

	for destination, nextCost := range nt.table {
		_, ok := ot.table[destination]
		cost := nextCost.cost + nt.table[ot.id].cost
		if ok {
			// If our cost is larger than the incoming cost, update our table.
			if ot.table[destination].cost > cost {
				ot.table[destination].cost = cost
				ot.table[destination].name = nID
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

	if r.id == watchTable {
		log.Println("		" + r.id + "		")
		log.Println("Destination | Next | Cost")
		for id, n := range r.table {
			log.Println("     " + id + "      " + "  " + n.name + "   " + "  " + strconv.Itoa(int(n.cost)) + "   ")
		}
		log.Println("")
	}
}

func (r *router) sendToNeighbors() {

	// Need lock here (was netmap lock)
	//r.lock.RLock()
	//defer r.lock.RUnlock()

	netMap.lock.RLock()
	defer netMap.lock.RUnlock()

	// Need to wait for other routers to begin listening on their channels
	// prior to sending the updated tables.
	time.Sleep(25 * time.Millisecond)

	for id, n1 := range r.table {
		if id == n1.name && id != r.id {

			msg := routerTable{
				id:    r.id,
				table: r.table,
			}

			netMap.ch[id] <- msg
		}
	}
}

func fastestPath(source, destination string) {

	// Need lock here (was netmap lock)
	netMap.lock.RLock()
	defer netMap.lock.RUnlock()

	r := netMap.routers[source]
	path := source
	cost := r.table[destination].cost

	for {
		// BUG: This prints twice for some reason.
		if source == destination {
			fmt.Println(path + " with delay = " + strconv.Itoa(int(cost)))
			return
		}

		// Next hop to get to the destination.
		source = r.table[destination].name
		r = netMap.routers[source]
		path = path + " -> " + source
	}
}

func routerThread(r router) {

	// Send my table to all my neighbors.
	go r.sendToNeighbors()

	for {
		select {
		case update := <-r.ch:
			r.UpdateTable(update)
			r.tableInfo()
		}
	}
}

func parseInput(file *os.File) {

	defer file.Close()
	scanner := bufio.NewScanner(file)
	neighbors := make(map[string][]neighbor)

	for scanner.Scan() {
		line := scanner.Text()
		res := strings.Split(line, " ")
		cost, _ := strconv.ParseInt(res[2], 10, 32)
		t := int32(cost)

		slice, ok := neighbors[res[0]]

		// If the slice doesn't exist, initialize the slice.
		if !ok {

			var newSlice []neighbor
			neighbors[res[0]] = newSlice
		}

		// Add the new neighbor to the router's slice.
		slice = append(slice, neighbor{
			name: res[1],
			cost: t,
		})
		neighbors[res[0]] = slice
	}

	// Create each router with their list of given neighbors.
	for r, n := range neighbors {
		go createRouter(r, n)
	}
}

func main() {

	f, err := os.Open("routers.txt")

	if err != nil {
		panic(err)
	}

	r := bufio.NewScanner(os.Stdin)

	fmt.Print("Which table would you like to watch update?: ")
	r.Scan()
	watchTable = r.Text()

	parseInput(f)
	time.Sleep(3 * time.Second)
	fmt.Println()
	fmt.Println("Compute the fastest path")
	fmt.Println("-----------------------------------")
	fmt.Print("Enter the source router: ")
	r.Scan()
	s := r.Text()
	fmt.Print("Enter the destination router: ")
	r.Scan()
	d := r.Text()
	fmt.Println()
	fastestPath(s, d)
	fmt.Println()
	fmt.Println()
	fmt.Println("Press ctrl+c to exit...")

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	<-ch
}
