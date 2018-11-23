package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
)

var TraitorGen int
var numGenerals int

const (
	A  = "A"
	R = "R"
)

type Node struct {
	inputValue  string
	outputValue string
	processIDs  map[int]int
	children    []*Node
	id          int
}

// This function will randomly select TraitorGen generals to be the Traitor ones.
// The commander can also be chosen.
func genTraitorIndexes(TraitorGen, numGenerals int) map[int]int {
	TraitorIndexes := make(map[int]int)
	p := rand.Perm(numGenerals)
	for _, r := range p[:TraitorGen] {
		TraitorIndexes[r] = 1
	}
	return TraitorIndexes
}

// This function will simulate sending a message by updating the information of the recieving node.
// It will then add the recieivng node as one of its children which will be used later in the decision.
func (node Node) sendMessage(id int, TraitorGenerals map[int]int) *Node {
	var order string
	// If this general has already recieved a message, don't send.
	if _, ok := node.processIDs[id]; ok {
		return nil
	}
	TraitorGeneral := false
	// Check if the general sending the node is Traitor
	if _, ok := TraitorGenerals[node.id]; ok {
		TraitorGeneral = true
	}
	if TraitorGeneral {
		if node.inputValue == R {
			order = A
		} else {
			order = R
		}
	} else {
		// else send order as is
		order = node.inputValue
	}
	processIDs := make(map[int]int)
	// send them all the process id's we have too
	for k, v := range node.processIDs {
		processIDs[k] = v
	}
	processIDs[id] = 1
	return &Node{order, "", processIDs, nil, id}
}

func (node *Node) decide() string {
	if len(node.children) == 0 {
		node.outputValue = node.inputValue
		return node.outputValue
	}
	decisions := make(map[string]int)
	for _, child := range node.children {
		decision := child.decide()
		if _, ok := decisions[decision]; ok {
			decisions[decision]++
		} else {
			decisions[decision] = 1
		}
	}
	if decisions[A] > decisions[R] {
		node.outputValue = A
	} else {
		node.outputValue = R
	}
	return node.outputValue
}

// This is the main orchestrator for the byzantine generals algorithm
func generals(numGenerals, TraitorGen int, order string) {
	//Randomly choose which generals are Traitor(can include the commander)
	TraitorIndexes := genTraitorIndexes(TraitorGen, numGenerals)

	// Initialize the commander with the original order and itself as a visited commander in the map.
	// This will ensure the commander is never messaged again.
	commander := &Node{order, "", map[int]int{0: 1}, nil, 0}
	if TraitorIndexes[0] == 1 {
		fmt.Println("Commander is Traitor")
	}

	queue := []*Node{commander}

	currDepth := 0
	elemDepth := 1
	nextElemDepth := 0

	// depth-limited breadth first search 
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		nextElemDepth += numGenerals - currDepth
		elemDepth--
		if elemDepth == 0 {
			currDepth++
			if currDepth >= TraitorGen {
				break
			}
			elemDepth = nextElemDepth
			nextElemDepth = 0
		}

		// Iterate through each general and send the message, add to end of BFS queue.
		for i := 1; i < numGenerals; i++ {
			childNode := node.sendMessage(i, TraitorIndexes)
			if childNode == nil {
				continue
			}
			node.children = append(node.children, childNode)
			queue = append(queue, childNode)
		}
	}

	commander.outputValue = commander.decide()

	for i, general := range commander.children {
		if _, ok := TraitorIndexes[i+1]; ok {
			fmt.Print("Traitor ")
		}
		if(general.outputValue == A) {
			fmt.Printf("General %d decides to ATTACK\n", general.id)
		} else {
			fmt.Printf("General %d decides to RETREAT\n", general.id)
		}
	}
	if(commander.outputValue == A) {
		fmt.Println("CONSENSUS IS ATTACK")
	} else {
		fmt.Println("CONSENSUS IS RETREAT")
	}
}

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Error. Usage: mybyz.go <m> <G> <A or R>")
		os.Exit(-1)
	}

	var err error

	TraitorGen, err = strconv.Atoi(os.Args[1])

	if err != nil {
		fmt.Println("Error, quitting...")
		os.Exit(-1)
	}

	numGenerals, err = strconv.Atoi(os.Args[2])

	if err != nil || TraitorGen >= numGenerals-1 {
		fmt.Println("Error on numGenerals input: Ensure that TraitorGen < numGenerals-1")
		os.Exit(-1)
	}

	var order string = os.Args[3]

	generals(numGenerals, TraitorGen, order)
}
