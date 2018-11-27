package kademlia

import (
	"fmt"
	"math/rand"
	"crypto/sha1"
	"net"
	"encoding/binary"
	"bytes"
	"encoding/hex"
	"container/heap";
 	"container/vector";
 	"log";
  	"http";
  	"os";
  	"rpc";
  	"sort";
)

const IdLength = 20
const BucketSize = 20;

type NodeID [IdLength]byte

type Contact struct {
  id NodeID;
}

type RoutingTable struct {
  node NodeID;
  buckets [IdLength*8]*list.List;
}

type ContactRecord struct {
  node *Contact;
  sortKey NodeID;
}

type Kademlia struct {
  routes *RoutingTable;
  NetworkId string;
}

type RPCHeader struct {
  Sender *Contact;
  NetworkId string;
}

type KademliaCore struct {
  kad *Kademlia;
}

type PingRequest struct {
  RPCHeader;
}

type PingResponse struct {
  RPCHeader;
}

type FindNodeRequest struct {
  RPCHeader;
  target NodeID;
}

type FindNodeResponse struct {
  RPCHeader;
  contacts []Contact;
}
// ----------node---------------------------------------------------------------------------------

func NewNodeID(data string) (ret NodeID) {
  decoded, _ := hex.DecodeString(data);
  for i := 0; i < IdLength; i++ {
    ret[i] = decoded[i];
  }
  return;
}

func NewRandomNodeID() (ret NodeID) {
  for i := 0; i < IdLength; i++ {
    ret[i] = uint8(rand.Intn(256));
  }
  return;
}

func (node NodeID) String() string {
  return hex.EncodeToString(node[0:IdLength]);
}

func (node NodeID) Equals(other NodeID) bool {
  for i := 0; i < IdLength; i++ {
    if node[i] != other[i] {
      return false;
    }
  }
  return true;
}

func (node NodeID) Less(other interface{}) bool {
  for i := 0; i < IdLength; i++ {
    if node[i] != other.(NodeID)[i] {
      return node[i] < other.(NodeID)[i];
    }
  }
  return false;
}

func (node NodeID) Xor(other NodeID) (ret NodeID) {
  for i := 0; i < IdLength; i++ {
    ret[i] = node[i] ^ other[i];
  }
  return;
}

func (node NodeID) PrefixLen() (ret int) {
  for i := 0; i < IdLength; i++ {
    for j := 0; j < 8; j++ {
      if (node[i] >> uint8(7 - j)) & 0x1 != 0 {
        return i * 8 + j;
      }
    }
  }
  return IdLength * 8 - 1;
}

//----------------------------------- node end ------------------------------------------------------------------

//---------------routing----------------------------------------------------------

func NewRoutingTable(node NodeID) (ret RoutingTable) {
  for i := 0; i < IdLength * 8; i++ {
    ret.buckets[i] = list.New();
  }
  ret.node = node;
  return;
}

func (table *RoutingTable) Update(contact *Contact) {
  prefix_length := contact.id.Xor(table.node.id).PrefixLen();
  bucket := table.buckets[prefix_length];
  element := iterable.Find(bucket, func(x interface{}) bool {
    return x.(*Contact).id.Equals(table.node.id);
  });
  if element == nil {
    if bucket.Len() <= BucketSize {
      bucket.PushFront(contact);
    }
    // TODO: Handle insertion when the list is full by evicting old elements if
    // they don't respond to a ping.
  } else {
    bucket.MoveToFront(element.(*list.Element));
  }
}

func (rec *ContactRecord) Less(other interface{}) bool {
  return rec.sortKey.Less(other.(*ContactRecord).sortKey);
}

func copyToVector(start, end *list.Element, vec *vector.Vector, target NodeID) {
  for elt := start; elt != end; elt = elt.Next() {
    contact := elt.Value.(*Contact);
    vec.Push(&ContactRecord{contact, contact.id.Xor(target)});
  }
}

func (table *RoutingTable) FindClosest(target NodeID, count int) (ret *vector.Vector) {
  ret = new(vector.Vector).Resize(0, count);

  bucket_num := target.Xor(table.node.id).PrefixLen();
  bucket := table.buckets[bucket_num];
  copyToVector(bucket.Front(), nil, ret, target);

  for i := 1; (bucket_num-i >= 0 || bucket_num+i < IdLength*8) && ret.Len() < count; i++ {
    if bucket_num - i >= 0 {
      bucket = table.buckets[bucket_num - i];
      copyToVector(bucket.Front(), nil, ret, target);
    }
    if bucket_num + i < IdLength * 8 {
      bucket = table.buckets[bucket_num + i];
      copyToVector(bucket.Front(), nil, ret, target);
    }
  }
  
  sort.Sort(ret);
  if ret.Len() > count {
    ret.Cut(count, ret.Len());
  }
  return;
}

// -----------end ------------------------------------------------------------------

//-------------kademlia --------------------------------------------------------

func NewKademlia(self *Contact, networkId string) (ret *Kademlia) {
  ret = new(Kademlia);
  ret.routes = NewRoutingTable(self);
  ret.NetworkId = networkId;
  return;
}

func (k *Kademlia) HandleRPC(request, response *RPCHeader) os.Error {
  if request.NetworkId != k.NetworkId {
    return os.NewError(fmt.Sprintf("Expected network ID %s, got %s",
                                   k.NetworkId, request.NetworkId));
  }
  if request.Sender != nil {
    k.routes.Update(request.Sender);
  }
  response.Sender = &k.routes.node;
  return nil;
}

func (kc *KademliaCore) Ping(args *PingRequest, response *PingResponse) (err os.Error) {
  if err = kc.kad.HandleRPC(&args.RPCHeader, &response.RPCHeader); err == nil {
    log.Stderr("Ping from %s\n", args.RPCHeader);
  }
  return;
}

func (k *Kademlia) Serve() (err os.Error) {
  rpc.Register(&KademliaCore{k});

  rpc.HandleHTTP();
  if l, err := net.Listen("tcp", k.routes.node.address); err == nil {
    go http.Serve(l, nil);
  }
  return;
}

func (kc *KademliaCore) FindNode(args *FindNodeRequest, response *FindNodeResponse) (err os.Error) {
  if err = kc.kad.HandleRPC(&args.RPCHeader, &response.RPCHeader); err == nil {
    contacts := kc.kad.routes.FindClosest(args.target, BucketSize);
    response.contacts = make([]Contact, contacts.Len());

    for i := 0; i < contacts.Len(); i++ {
      response.contacts[i] = *contacts.At(i).(*ContactRecord).node;
    }
  }
  return;
}

func (k *Kademlia) IterativeFindNode(target NodeID, delta int) (ret *vector.Vector) {
  done := make(chan []Contact);

  // A vector of *ContactRecord structs
  ret = new(vector.Vector).Resize(0, BucketSize);
  
  // A heap of not-yet-queried *Contact structs
  frontier := new(vector.Vector).Resize(0, BucketSize);
  
  // A map of client values we've seen so far
  seen := make(map[string] bool);
  
  // Initialize the return list, frontier heap, and seen list with local nodes
  for node := range k.routes.FindClosest(target, delta).Iter() {
    record := node.(*ContactRecord);
    ret.Push(record);
    heap.Push(frontier, record.node);
    seen[record.node.id.String()] = true;
  }
  
  // Start off delta queries
  pending := 0;
  for i := 0; i < delta && frontier.Len() > 0; i++ {
    pending++;
    go k.sendQuery(frontier.Pop().(*Contact), target, done);
  }
  
  // Iteratively look for closer nodes
  for pending > 0 {
    nodes := <-done;
    pending--;
    for _, node := range nodes {
      // If we haven't seen the node before, add it
      if _, ok := seen[node.id.String()]; ok == false {
        ret.Push(&ContactRecord{&node, node.id.Xor(target)});
        heap.Push(frontier, node);
        seen[node.id.String()] = true;
      }
    }
    
    for pending < delta && frontier.Len() > 0 {
      go k.sendQuery(frontier.Pop().(*Contact), target, done);
      pending++;
    }
  }

  sort.Sort(ret);
  if ret.Len() > BucketSize {
    ret.Cut(BucketSize, ret.Len());
  }

  return;
}

func (k *Kademlia) Call(contact *Contact, method string, args, reply interface{}) (err os.Error) {
  if client, err := rpc.DialHTTP("tcp", contact.address); err == nil {
    err = client.Call(method, args, reply);
    if err == nil {
      k.routes.Update(contact);
    }
  }
  return;
}

func (k *Kademlia) sendQuery(node *Contact, target NodeID, done chan []Contact) {
  args := FindNodeRequest{RPCHeader{&k.routes.node, k.NetworkId}, target};
  reply := FindNodeResponse{};
  
  if err := k.Call(node, "KademliaCore.FindNode", &args, &reply); err == nil {
    done <- reply.contacts;
  } else {
    done <- []Contact{};
  }
}
