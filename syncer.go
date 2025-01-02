package main

type Consumer func(syncEvent SyncEvent)

type Syncer interface {
	Sync(ch chan SyncEvent)
	Consume(ch chan SyncEvent, consumers ...Consumer)
	//Rollback(commitId string, ch chan SyncEvent)
}

type SyncEvent struct {
	Data struct {
		Progress  int    `json:"progress"`
		IsSuccess bool   `json:"isSuccess"`
		Step      string `json:"step"`
		Error     string `json:"error"`
		Done      bool   `json:"done"`
	} `json:"data"`
}
