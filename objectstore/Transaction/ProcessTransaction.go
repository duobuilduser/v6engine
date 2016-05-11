package Transaction

import (
	"duov6.com/objectstore/cache"
	"duov6.com/objectstore/messaging"
	"duov6.com/objectstore/repositories"
	"duov6.com/objectstore/storageengines"
	"encoding/json"
	"errors"
	"strconv"
)

func Execute(request *messaging.ObjectRequest) (err error) {
	//Get Length of List
	TransactionID := request.Body.Transaction.Parameters["TransactionID"].(string)
	if cache.ExistsKeyValue(request, GetBucketName(TransactionID)) && cache.GetListLength(request, GetBucketName(TransactionID)) > 1 {
		err = StartProcess(request)
	} else {
		err = errors.New("Transaction either already Rolledback or no transaction items found!")
	}
	return
}

func StartProcess(request *messaging.ObjectRequest) (err error) {
	//GetTask
	TransactionID := request.Body.Transaction.Parameters["TransactionID"].(string)
	tasklength := cache.GetListLength(request, GetBucketName(TransactionID))

	var x int64

	for x = 0; x < tasklength-1; x++ {
		pickedRequest, err2 := GetTask(request)
		if err2 != nil {
			//Rollback executed while executing last processs -> Execute rollback process
			err = StartRollBackProcess(request)
			return
		} else {
			//execute
			response := ProcessDispatcher(pickedRequest)
			//if success -> Push to success list, Create invert request and push to invert list
			if response.IsSuccess {
				_ = PushToSuccessList(pickedRequest, TransactionID)
				//get inverted request
				invertedRequest := GetInvertedRequest(pickedRequest)
				_ = PushToInvertList(invertedRequest, TransactionID)
			} else { //if false -> Start rollback process
				err = StartRollBackProcess(request)
				return
			}
		}

	}
	return
}

func ProcessDispatcher(request *messaging.ObjectRequest) repositories.RepositoryResponse {

	var storageEngine storageengines.AbstractStorageEngine // request.StoreConfiguration.StorageEngine
	storageEngine = storageengines.ReplicatedStorageEngine{}

	var outResponse repositories.RepositoryResponse = storageEngine.Store(request)

	if request.IsLogEnabled {
		for index, element := range request.MessageStack {
			request.Log("S-" + strconv.Itoa(index) + " : " + element)
		}
	}

	return outResponse
}

func GetTask(request *messaging.ObjectRequest) (retRequest *messaging.ObjectRequest, err error) {
	TransactionID := request.Body.Transaction.Parameters["TransactionID"].(string)
	var byteVal []byte
	byteVal, err = cache.LPop(request, GetBucketName(TransactionID))

	// if err != nil -> key has removed.. RollBack has been called

	if err == nil {
		err2 := json.Unmarshal(byteVal, &request)
		if err2 != nil {
			request.Log(err2.Error())
		}
	}
	return
}

func GetInvertedTask(request *messaging.ObjectRequest) (retRequest *messaging.ObjectRequest, err error) {
	TransactionID := request.Body.Transaction.Parameters["TransactionID"].(string)
	var byteVal []byte
	byteVal, err = cache.LPop(request, GetInvertBucketName(TransactionID))

	// if err != nil -> key has removed.. RollBack has been called

	if err == nil {
		err2 := json.Unmarshal(byteVal, &request)
		if err2 != nil {
			request.Log(err2.Error())
		}
	}
	return
}

func PushToSuccessList(request *messaging.ObjectRequest, TransactionID string) (err error) {
	bucketValue, err := json.Marshal(request)
	err = cache.RPush(request, GetSuccessBucketName(TransactionID), string(bucketValue))
	return
}

func PushToInvertList(request *messaging.ObjectRequest, TransactionID string) (err error) {
	bucketValue, err := json.Marshal(request)
	err = cache.RPush(request, GetInvertBucketName(TransactionID), string(bucketValue))
	return
}

func StartRollBackProcess(request *messaging.ObjectRequest) (err error) {

	TransactionID := request.Body.Transaction.Parameters["TransactionID"].(string)

	tasklength := cache.GetListLength(request, GetInvertBucketName(TransactionID))
	isAllSuccess := true

	var x int64

	for x = 0; x < tasklength; x++ {
		pickedRequest, _ := GetInvertedTask(request)
		response := ProcessDispatcher(pickedRequest)
		if !response.IsSuccess {
			isAllSuccess = false
		}
	}

	if !isAllSuccess {
		err = errors.New("Not All Rollbacks were successful!")
	}

	return
}
