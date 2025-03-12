package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"
)

type IPDetail struct {
	Accuracy  string `json:"accuracy"`
	Adcode    string `json:"adcode"`
	Areacode  string `json:"areacode"`
	Asnumber  string `json:"asnumber"`
	City      string `json:"city"`
	Continent string `json:"continent"`
	Country   string `json:"country"`
	Isp       string `json:"isp"`
	Latwgs    string `json:"latwgs"`
	Lngwgs    string `json:"lngwgs"`
	Owner     string `json:"owner"`
	Province  string `json:"province"`
	Radius    string `json:"radius"`
	Source    string `json:"source"`
	Timezone  string `json:"timezone"`
	Zipcode   string `json:"zipcode"`
}

type Model struct {
	Type           string `json:"type"`
	UUID           string `json:"uuid"`
	DistinctID     string `json:"distinct_id"`
	EventName      string `json:"event_name"`
	Time           int64  `json:"time"`
	ZoneOffset     int    `json:"zone_offset"`
	NetworkType    string `json:"network_type"`
	Carrier        string `json:"carrier"`
	AppVersion     string `json:"app_version"`
	OSVersion      string `json:"os_version"`
	LibVersion     string `json:"lib_version"`
	SystemLanguage string `json:"system_language"`
	CPU            string `json:"cpu"`
	RAM            string `json:"ram"`
	Disk           string `json:"disk"`
	FPS            int    `json:"fps"`
	Properties     string `json:"properties"`
}

type MyData struct {
	DeviceID     string   `json:"device_id"`
	AppID        string   `json:"app_id"`
	InstallTime  int64    `json:"install_time"`
	OS           string   `json:"os"`
	ScreenWidth  int      `json:"screen_width"`
	ScreenHeight int      `json:"screen_height"`
	DeviceModel  string   `json:"device_model"`
	DeviceType   string   `json:"device_type"`
	BundleID     string   `json:"bundle_id"`
	Manufacturer string   `json:"manufacturer"`
	Stime        int64    `json:"stime"`
	IP           string   `json:"ip"`
	IPDetail     IPDetail `json:"ip_detail"`
	Model        []Model  `json:"model"`
}

// dataPool with a counter.
type countedDataPool struct {
	pool  sync.Pool
	count int64 // Use an atomic counter
}

func (p *countedDataPool) Get() *MyData {
	if p.count == 0 {
		return datagen()
	}

	atomic.AddInt64(&p.count, -1) // Decrement count when taking from pool
	return p.pool.Get().(*MyData)
}

func (p *countedDataPool) Put(data *MyData) {
	p.pool.Put(data)
	atomic.AddInt64(&p.count, 1) // Increment count when returning to pool
}

func (p *countedDataPool) Len() int64 {
	return atomic.LoadInt64(&p.count) // Non-blocking read of the counter
}

var dataPool = countedDataPool{
	pool: sync.Pool{
		New: func() any { return &MyData{} },
	},
	count: 0, // Explicitly initialize count to 0.
}

// Global counter for total messages published
var totalMessagesPublished int64 = 0

func datagen() *MyData {
	// Get a MyData object from the pool, or create a new one if the pool is empty.
	data := &MyData{}

	data.DeviceID = randomString(36)
	data.AppID = randomString(32)
	data.InstallTime = rand.Int63()
	data.OS = "iOS"
	data.ScreenWidth = rand.Intn(1000)
	data.ScreenHeight = rand.Intn(2000)
	data.DeviceModel = "iPhone15,5"
	data.DeviceType = "iPhone"
	data.BundleID = "com.example.app"
	data.Manufacturer = "Apple"
	data.Stime = rand.Int63()
	data.IP = "192.168.1.1"
	data.IPDetail = IPDetail{
		Accuracy:  "城市",
		Adcode:    "",
		Areacode:  "GB",
		Asnumber:  "5089",
		City:      "伦敦",
		Continent: "欧洲",
		Country:   "英国",
		Isp:       "维珍传媒有限公司",
		Latwgs:    "51.513816",
		Lngwgs:    "-0.121887",
		Owner:     "JARROW",
		Province:  "英格兰",
		Radius:    "",
		Source:    "数据挖掘",
		Timezone:  "UTC+0",
		Zipcode:   "WC2B 5QZ",
	}
	data.Model = []Model{
		{
			Type:           "track",
			UUID:           randomString(36),
			DistinctID:     randomString(36),
			EventName:      "app_start_detail",
			Time:           rand.Int63(),
			ZoneOffset:     1,
			NetworkType:    "WIFI",
			Carrier:        "--",
			AppVersion:     "4.6.7",
			OSVersion:      "18.0.1",
			LibVersion:     "1.0.0.0",
			SystemLanguage: "en",
			CPU:            "arm64e",
			RAM:            "1.0/5.5",
			Disk:           "15.8/119.1",
			FPS:            50,
			Properties:     `{"#device_id":"B4FB831D-2C2E-4A16-9D28-2CD927F565BC","#lib":"iOS","time":1728489599.611932,"s_coldstratnum_V421":196,"session_id":"AE5C57D2-C78E-4C6B-AA18-CDE2421E4A79","operation_type":3,"#simulator":false,"data_ad_waynum":{"1":"4337","aggregtion":"IS","pici":"43-2"}}`,
		},
	}

	return data
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// addCommas adds commas to an integer string.
func addCommas(n int64) string {
	in := fmt.Sprintf("%d", n)
	var out []byte
	l := len(in)
	for i := 0; i < l; i++ {
		if i > 0 && (l-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, in[i])
	}
	return string(out)
}

func main() {
	// Create a context that can be canceled for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure context is canceled in any case

	// Replace with your project ID and topic ID
	projectID := "du-hast-mich"

	topicName := "dingo-topic"
	serviceAccountKeyFilePath := "/usr/local/google/home/binwu/workspace/google/sa.json" // if you need to use service account

	// Create a Pub/Sub client.
	// if you need to use service account, use the following code
	client, err := pubsub.NewClient(ctx, projectID,
		option.WithCredentialsFile(serviceAccountKeyFilePath))
	//client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create Pub/Sub client: %v", err)
	}
	defer client.Close()

	topic := client.Topic(topicName)
	defer topic.Stop()

	// Status reporting goroutine (non-blocking)
	go func() {
		for {
			select {
			case <-ctx.Done():
				// Context canceled, exit the goroutine
				return
			default:
				poolLen := dataPool.Len()
				totalSent := atomic.LoadInt64(&totalMessagesPublished) // Non-blocking read

				// log.Printf("DataPool size: %d, Total messages published: %d", poolLen, totalSent)
				log.Printf("DataPool size: %s, Total messages published: %s", addCommas(poolLen), addCommas(totalSent))

				// Use a ticker with select to make it responsive to shutdown
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
					// Continue the loop
				}
			}
		}
	}()

	// numPublishers := runtime.NumCPU()     // threads for publishing message to Kafka from channels
	// numDataGenThreads := runtime.NumCPU() // threads for generating data and fill the dataPool
	// numWorkers := runtime.NumCPU()        // threads for consuming data from dataPool and publish to channels
	numPublishers := 2
	numDataGenThreads := 2
	numWorkers := 2

	// Prefill the dataPool before starting the publishers
	const poolSize = 999990

	const prefillSize = poolSize / 10
	log.Printf("Prefilling the dataPool with %s items...", addCommas(prefillSize))
	for i := 0; i < prefillSize; i++ {
		dataPool.Put(datagen())
	}
	log.Printf("DataPool prefill complete.  Current size: %s", addCommas(dataPool.Len()))

	const bufferSize = 99999

	// Create a dedicated channel for EACH publisher.
	publisherChs := make([]chan *pubsub.Message, numPublishers)
	for i := 0; i < numPublishers; i++ {
		publisherChs[i] = make(chan *pubsub.Message, bufferSize)
	}

	// Data Generation Goroutines
	var wgDataGen sync.WaitGroup
	wgDataGen.Add(numDataGenThreads)
	for i := 0; i < numDataGenThreads; i++ {
		go func(threadID int) {
			defer wgDataGen.Done()
			for {
				select {
				case <-ctx.Done():
					// Context canceled, exit the goroutine immediately
					log.Printf("Thread %d: Data generation stopping due to shutdown signal", threadID)
					return
				default:
					// Check if the pool is full
					if dataPool.Len() >= poolSize { // Pool is full
						log.Printf("Thread %d: DataPool is full, current size: %s, Waiting...",
							threadID, addCommas(dataPool.Len()))
						// Use a ticker with select to make it responsive to shutdown
						select {
						case <-ctx.Done():
							return
						case <-time.After(1000 * time.Millisecond):
							// Continue the loop
						}
						continue // Skip this iteration
					}

					// Put the item into the pool.
					dataPool.Put(datagen())
				}
			}
		}(i)
	}

	// Data Consumption and fill channels Goroutines
	var wgWorkers sync.WaitGroup
	wgWorkers.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(publisherChID int, ch chan *pubsub.Message) {
			defer wgWorkers.Done()
			for { // Loop to continuously consume from dataPool and produce to Pub/Sub
				select {
				case <-ctx.Done():
					// Context canceled, exit the goroutine
					log.Printf("Worker %d: Stopping due to shutdown signal", publisherChID)
					return
				default:
					item := dataPool.Get()

					jsonData, err := json.Marshal(item)
					if err != nil {
						log.Printf("Publisher %d: Failed to marshal JSON: %v", publisherChID, err)
						continue
					}

					msg := &pubsub.Message{
						Data: jsonData,
					}

					// Non-blocking send to Pub/Sub channel with retry on full queue
					select {
					case <-ctx.Done():
						// Put the item back in the pool if we're shutting down
						//dataPool.Put(item)
						return
					case ch <- msg: // Try to send to Pub/Sub
						// Successfully sent to channel
					default:
						// Pub/Sub channel is full. Wait and retry.
						log.Println("Publisher channel is full, waiting...")
						select {
						case <-ctx.Done():
							// Put the item back in the pool if we're shutting down
							//dataPool.Put(item)
							return
						case <-time.After(1 * time.Second):
							// Try again after waiting
						}
						// Try again after the wait
						select {
						case <-ctx.Done():
							//dataPool.Put(item)
							return
						case ch <- msg: // Requeue on the *same* channel. ignore if channel is full
							// Successfully sent
						}
					}
				}
			}
		}(i, publisherChs[i]) // Pass the dedicated channel
	}

	// Pub/Sub Producer Goroutines
	var wgPublishers sync.WaitGroup
	wgPublishers.Add(numPublishers)
	for i := 0; i < numPublishers; i++ {
		go func(publisherID int, ch chan *pubsub.Message) {
			defer wgPublishers.Done()
			for msg := range ch {
				res := topic.Publish(ctx, msg)
				_, err := res.Get(ctx)
				if err != nil {
					log.Printf("Publisher %d: Failed to publish message: %v", publisherID, err)
					// No retry here, just log and continue.
				} else {
					atomic.AddInt64(&totalMessagesPublished, 1)
				}

				// Use a ticker with select to make it responsive to shutdown
				select {
				case <-ctx.Done():
					// If shutting down, process remaining messages faster
				case <-time.After(100 * time.Millisecond):
					// Normal rate limiting
				}
			}
			log.Printf("Publisher %d: Finished processing all queued messages", publisherID)
		}(i, publisherChs[i]) // Pass the dedicated channel to each publisher
	}

	// Handle SIGINT and SIGTERM for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received
	sig := <-sigChan
	log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)

	// Step 1: Cancel the context to stop data generation immediately
	cancel()
	log.Println("Signaled all goroutines to stop")

	// Step 2: Wait for data generation to finish
	log.Println("Waiting for data generation goroutines to stop...")
	wgDataGen.Wait()
	log.Println("Data generation stopped successfully")

	// Step 3: Wait for workers to finish
	log.Println("Waiting for worker goroutines to finish...")
	wgWorkers.Wait()
	log.Println("Worker goroutines stopped successfully")

	// Step 4: Close all publisher channels to signal publisher goroutines to exit
	log.Println("Closing publisher channels...")
	for i, ch := range publisherChs {
		log.Printf("Closing publisher channel %d", i)
		close(ch)
	}

	// Step 5: Wait for publishers to finish processing remaining messages
	log.Println("Waiting for publishers to finish processing remaining messages...")
	wgPublishers.Wait()
	log.Println("All publishers finished successfully")

	// Final cleanup
	log.Println("Graceful shutdown complete.")
	os.Exit(0)
}
