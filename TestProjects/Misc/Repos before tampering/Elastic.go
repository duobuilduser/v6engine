package repositories

import (
	"duov6.com/common"
	"duov6.com/objectstore/cache"
	"duov6.com/objectstore/keygenerator"
	"duov6.com/objectstore/messaging"
	"duov6.com/queryparser"
	"duov6.com/term"
	"encoding/json"
	"github.com/mattbaird/elastigo/lib"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ElasticRepository struct {
}

func (repository ElasticRepository) GetRepositoryName() string {
	return "Elastic Search"
}

func (repository ElasticRepository) GetAll(request *messaging.ObjectRequest) RepositoryResponse {
	request.Log("Starting GETALL")
	return repository.search(request, "*")
}

func (repository ElasticRepository) GetSearch(request *messaging.ObjectRequest) RepositoryResponse {
	request.Log("Starting GETSEARCH")
	return repository.search(request, request.Body.Query.Parameters)
}

func (repository ElasticRepository) search(request *messaging.ObjectRequest, searchStr string) RepositoryResponse {
	response := RepositoryResponse{}
	conn := repository.getConnection(request)

	skip := "0"

	if request.Extras["skip"] != nil {
		skip = request.Extras["skip"].(string)
	}

	take := "100"

	if request.Extras["take"] != nil {
		take = request.Extras["take"].(string)
	}

	orderbyfield := ""
	var query string

	if request.Extras["orderby"] != nil {
		orderbyfield = request.Extras["orderby"].(string)
		operator := "asc"
		query = "{\"sort\" : [{\"" + orderbyfield + "\" : {\"order\" : \"" + operator + "\"}}],\"from\": " + skip + ", \"size\": " + take + ", \"query\":{\"query_string\" : {\"analyze_wildcard\": true, \"query\" : \"" + searchStr + "\"}}}"
	} else if request.Extras["orderbydsc"] != nil {
		orderbyfield = request.Extras["orderbydsc"].(string)
		operator := "desc"
		query = "{\"sort\" : [{\"" + orderbyfield + "\" : {\"order\" : \"" + operator + "\"}}],\"from\": " + skip + ", \"size\": " + take + ", \"query\":{\"query_string\" : {\"analyze_wildcard\": true, \"query\" : \"" + searchStr + "\"}}}"
	} else {
		query = "{\"from\": " + skip + ", \"size\": " + take + ", \"query\":{\"query_string\" : {\"analyze_wildcard\": true, \"query\" : \"" + searchStr + "\"}}}"
	}

	isSearchGlobalNamespace := false
	if request.Extras["searchGlobalNamespace"] != nil {
		if strings.EqualFold(request.Extras["searchGlobalNamespace"].(string), "TRUE") {
			term.Write("Global Search Enabled!", term.Debug)
			isSearchGlobalNamespace = true
		}
	}

	var err error
	var data elastigo.SearchResult

	term.Write("Elastic Query : ", term.Debug)
	term.Write(query, term.Debug)

	if isSearchGlobalNamespace {
		data, err = conn.Search(request.Controls.Namespace, "", nil, query)
	} else {
		data, err = conn.Search(request.Controls.Namespace, request.Controls.Class, nil, query)
	}

	if err != nil {
		response.GetResponseWithBody(getEmptyByteObject())
	} else {
		var allMaps []map[string]interface{}
		allMaps = make([]map[string]interface{}, data.Hits.Len())
		for index, hit := range data.Hits.Hits {
			var currentMap map[string]interface{}
			currentMap = make(map[string]interface{})

			byteData, _ := hit.Source.MarshalJSON()
			json.Unmarshal(byteData, &currentMap)
			delete(currentMap, "__osHeaders")

			// if currentMap["workActionID"] != nil {
			// 	currentMap["workActionID"] = hit.Id
			// }

			allMaps[index] = currentMap
		}

		finalBytes, _ := json.Marshal(allMaps)
		response.GetResponseWithBody(finalBytes)
	}

	return response
}

func (repository ElasticRepository) GetQuery(request *messaging.ObjectRequest) RepositoryResponse {
	request.Log("Starting GET-QUERY!")
	response := RepositoryResponse{}
	queryType := request.Body.Query.Type

	switch queryType {
	case "Query":
		if request.Body.Query.Parameters != "*" {
			fieldsInByte := repository.executeQuery(request)
			if fieldsInByte != nil {
				response.IsSuccess = true
				response.Message = "Successfully Retrieved Data For Custom Query"
				response.GetResponseWithBody(fieldsInByte)
			} else {
				response.IsSuccess = false
				response.Message = "Aborted! Unsuccessful Retrieving Data For Custom Query"
				errorMessage := response.Message
				response.GetErrorResponse(errorMessage)
			}
		} else {
			return repository.search(request, request.Body.Query.Parameters)
		}
	default:
		return repository.search(request, request.Body.Query.Parameters)

	}

	return response
}

func (repository ElasticRepository) GetByKey(request *messaging.ObjectRequest) RepositoryResponse {
	request.Log("Starting GET-BY-KEY")
	response := RepositoryResponse{}
	conn := repository.getConnection(request)

	key := getNoSqlKey(request)
	data, err := conn.Get(request.Controls.Namespace, request.Controls.Class, key, nil)

	if err != nil {
		response.GetResponseWithBody(getEmptyByteObject())
	} else {
		bytes, err := data.Source.MarshalJSON()
		//Get Data to struct
		var originalData map[string]interface{}
		originalData = make(map[string]interface{})
		json.Unmarshal(bytes, &originalData)
		delete(originalData, "__osHeaders")

		bytes, err = json.Marshal(originalData)
		if err != nil {
			errorMessage := "Elastic search JSON marshal error : " + err.Error()
			response.GetErrorResponse(errorMessage)

		} else {
			response.GetResponseWithBody(bytes)
		}

	}

	return response
}

func (repository ElasticRepository) InsertMultiple(request *messaging.ObjectRequest) RepositoryResponse {
	request.Log("Starting INSERT-MULTIPLE")
	return repository.setManyElastic(request)
}

func (repository ElasticRepository) InsertSingle(request *messaging.ObjectRequest) RepositoryResponse {
	request.Log("Starting INSERT-SINGLE")
	return repository.setOneElastic(request)
}

func (repository ElasticRepository) setOneElastic(request *messaging.ObjectRequest) RepositoryResponse {
	response := RepositoryResponse{}
	conn := repository.getConnection(request)

	key := ""
	id := ""
	if request.Body.Object["OriginalIndex"] != nil {
		key = request.Body.Object["OriginalIndex"].(string)
	} else {
		id = repository.getRecordID(request, request.Body.Object)
		request.Body.Object[request.Body.Parameters.KeyProperty] = id
		key = request.Controls.Namespace + "." + request.Controls.Class + "." + id
	}
	_, err := conn.Index(request.Controls.Namespace, request.Controls.Class, key, nil, request.Body.Object)
	if err != nil {
		errorMessage := "Elastic Search Single Insert Error : " + err.Error()
		response.GetErrorResponse(errorMessage)
		return response
	} else {
		response.IsSuccess = true
		response.Message = "Successfully inserted one to elastic search"
	}

	//Update Response
	var Data []map[string]interface{}
	Data = make([]map[string]interface{}, 1)
	var actualData map[string]interface{}
	actualData = make(map[string]interface{})
	actualData["ID"] = id
	Data[0] = actualData
	response.Data = Data
	return response
}

func (repository ElasticRepository) setManyElastic(request *messaging.ObjectRequest) RepositoryResponse {
	response := RepositoryResponse{}

	conn := repository.getConnection(request)

	var Data map[string]interface{}
	Data = make(map[string]interface{})

	for index, obj := range request.Body.Objects {
		id := repository.getRecordID(request, obj)
		Data[strconv.Itoa(index)] = id
		request.Body.Objects[index][request.Body.Parameters.KeyProperty] = id
	}

	noOfElementsPerSet := 500
	noOfSets := (len(request.Body.Objects) / noOfElementsPerSet)
	remainderFromSets := 0
	remainderFromSets = (len(request.Body.Objects) - (noOfSets * noOfElementsPerSet))

	startIndex := 0
	stopIndex := noOfElementsPerSet

	var status []bool

	if remainderFromSets == 0 {
		status = make([]bool, noOfSets)
	} else {
		status = make([]bool, (noOfSets + 1))
	}

	statusIndex := 0

	for x := 0; x < noOfSets; x++ {
		tempStatus := repository.insertRecordStub(request, request.Body.Objects[startIndex:stopIndex], conn)
		status[statusIndex] = tempStatus

		if tempStatus {
			term.Write("Inserted Stub : "+strconv.Itoa(statusIndex), term.Debug)
		} else {
			term.Write("Inserting Failed Stub : "+strconv.Itoa(statusIndex), term.Debug)
		}

		statusIndex += 1
		startIndex += noOfElementsPerSet
		stopIndex += noOfElementsPerSet

		time.Sleep(1 * time.Millisecond)

	}

	if remainderFromSets > 0 {
		start := len(request.Body.Objects) - remainderFromSets

		tempStatus := repository.insertRecordStub(request, request.Body.Objects[start:len(request.Body.Objects)], conn)
		status[statusIndex] = tempStatus

		if tempStatus {
			term.Write("Inserted Stub : "+strconv.Itoa(statusIndex), term.Debug)
		} else {
			term.Write("Inserting Failed Stub : "+strconv.Itoa(statusIndex), term.Debug)
		}

		statusIndex += 1
	}

	isAllCompleted := true

	for _, val := range status {
		if !val {
			isAllCompleted = false
			break
		}
	}

	if isAllCompleted {
		response.IsSuccess = true
		response.Message = "Successfully inserted bulk to Elastic Search"
	} else {
		response.IsSuccess = false
		response.Message = "Error Inserting Some Objects"
	}

	//Update Response
	var DataMap []map[string]interface{}
	DataMap = make([]map[string]interface{}, 1)
	var actualInput map[string]interface{}
	actualInput = make(map[string]interface{})
	actualInput["ID"] = Data
	DataMap[0] = actualInput
	response.Data = DataMap
	return response
}

func (repository ElasticRepository) insertRecordStub(request *messaging.ObjectRequest, records []map[string]interface{}, conn *elastigo.Conn) (status bool) {
	status = true
	indexer := conn.NewBulkIndexerErrors(1000, 60)
	indexer.Start()
	for _, obj := range records {
		nosqlid := ""
		if obj["OriginalIndex"] != nil {
			nosqlid = obj["OriginalIndex"].(string)
		} else {
			nosqlid = getNoSqlKeyById(request, obj)
		}
		indexer.Index(request.Controls.Namespace, request.Controls.Class, nosqlid, "", "", nil, obj)
	}
	indexer.Stop()

	return

}

func (repository ElasticRepository) UpdateMultiple(request *messaging.ObjectRequest) RepositoryResponse {
	request.Log("Starting UPDATE-MULTIPLE")
	return repository.setManyElastic(request)
}

func (repository ElasticRepository) UpdateSingle(request *messaging.ObjectRequest) RepositoryResponse {
	request.Log("Starting UPDATE-SINGLE")
	return repository.setOneElastic(request)
}

func (repository ElasticRepository) DeleteMultiple(request *messaging.ObjectRequest) RepositoryResponse {
	request.Log("Starting DELETE-MULTIPLE")
	response := RepositoryResponse{}

	conn := repository.getConnection(request)

	for _, object := range request.Body.Objects {
		key := getNoSqlKeyById(request, object)
		_, err := conn.Delete(request.Controls.Namespace, request.Controls.Class, key, nil)
		if err != nil {
			errorMessage := "Elastic Search single delete error : " + err.Error()
			response.GetErrorResponse(errorMessage)
		} else {
			response.IsSuccess = true
			response.Message = "Successfully deleted one in elastic search"
		}
	}
	return response

}

func (repository ElasticRepository) DeleteSingle(request *messaging.ObjectRequest) RepositoryResponse {
	request.Log("Starting DELETE-SINGLE")
	response := RepositoryResponse{}

	conn := repository.getConnection(request)
	_, err := conn.Delete(request.Controls.Namespace, request.Controls.Class, getNoSqlKey(request), nil)
	if err != nil {
		errorMessage := "Elastic Search single delete error : " + err.Error()
		request.Log(errorMessage)
		response.GetErrorResponse(errorMessage)
	} else {
		response.IsSuccess = true
		response.Message = "Successfully deleted one in elastic search"
	}

	return response
}

func (repository ElasticRepository) Special(request *messaging.ObjectRequest) RepositoryResponse {
	response := RepositoryResponse{}
	request.Log("Starting SPECIAL!")
	queryType := strings.ToLower(request.Body.Special.Type)

	switch queryType {
	case "getfields":
		request.Log("Starting GET-FIELDS sub routine!")
		fieldsInByte := repository.executeGetFields(request)
		if fieldsInByte != nil {
			response.IsSuccess = true
			response.Message = "Successfully Retrieved Fileds on Class : " + request.Controls.Class
			response.GetResponseWithBody(fieldsInByte)
		} else {
			response.IsSuccess = false
			response.Message = "Aborted! Unsuccessful Retrieving Fileds on Class : " + request.Controls.Class
			errorMessage := response.Message
			response.GetErrorResponse(errorMessage)
		}
	case "getclasses":
		request.Log("Starting GET-CLASSES sub routine")
		fieldsInByte := repository.executeGetClasses(request)
		if fieldsInByte != nil {
			response.IsSuccess = true
			response.Message = "Successfully Retrieved Fileds on Class : " + request.Controls.Class
			response.GetResponseWithBody(fieldsInByte)
		} else {
			response.IsSuccess = false
			response.Message = "Aborted! Unsuccessful Retrieving Fileds on Class : " + request.Controls.Class
			errorMessage := response.Message
			response.GetErrorResponse(errorMessage)
		}
	case "getnamespaces":
		request.Log("Starting GET-NAMESPACES sub routine")
		fieldsInByte := repository.executeGetNamespaces(request)
		if fieldsInByte != nil {
			response.IsSuccess = true
			response.Message = "Successfully Retrieved All Namespaces"
			response.GetResponseWithBody(fieldsInByte)
		} else {
			response.IsSuccess = false
			response.Message = "Aborted! Unsuccessful Retrieving All Namespaces"
			errorMessage := response.Message
			response.GetErrorResponse(errorMessage)
		}
	case "getselected":
		request.Log("Starting GET-SELECTED_FIELDS sub routine")
		fieldsInByte := repository.executeGetSelectedFields(request)
		if fieldsInByte != nil {
			response.IsSuccess = true
			response.Message = "Successfully Retrieved All selected Field data"
			response.GetResponseWithBody(fieldsInByte)
		} else {
			response.IsSuccess = false
			response.Message = "Aborted! Unsuccessful Retrieving All selected field data"
			errorMessage := response.Message
			response.GetErrorResponse(errorMessage)
		}
	case "dropclass":
		request.Log("Starting Drop-Class sub routine")
		err := repository.executeDropClass(request)
		if err != nil {
			response.IsSuccess = false
			response.Message = "Aborted! Unsuccessful deleting Class!"
		} else {
			response.IsSuccess = true
			response.Message = "Aborted! Successful deleting Class!"
		}
	case "dropnamespace":
		request.Log("Starting Drop-Namespace sub routine")
		err := repository.executeDropNamespace(request)
		if err != nil {
			response.IsSuccess = false
			response.Message = "Aborted! Unsuccessful deleting Namespace!"
		} else {
			response.IsSuccess = true
			response.Message = "Successfully deleted Namespace!"
		}
	case "flushcache":
		if CheckRedisAvailability(request) {
			cache.FlushCache(request)
		}
		response.IsSuccess = true
		response.Message = "Cache Cleared successfully!"
	case "idservice":
		var IsPattern bool
		var idServiceCommand string

		if request.Body.Special.Extras["Pattern"] != nil {
			IsPattern = request.Body.Special.Extras["Pattern"].(bool)
		}

		if request.Body.Special.Extras["Command"] != nil {
			idServiceCommand = strings.ToLower(request.Body.Special.Extras["Command"].(string))
		}

		switch idServiceCommand {
		case "getid":
			if IsPattern {
				//pattern code goes here
				prefix, valueInString := keygenerator.GetPatternAttributes(request)
				var value int
				value, _ = strconv.Atoi(valueInString)

				if CheckRedisAvailability(request) {
					id := keygenerator.GetIncrementID(request, "ELASTIC", value)

					for x := 0; x < len(request.Controls.Class); x++ {
						if (len(prefix) + len(id)) < len(request.Controls.Class) {
							prefix += "0"
						} else {
							break
						}
					}

					id = prefix + id
					response.Body = []byte(id)
					response.IsSuccess = true
					response.Message = "Successfully Completed!"
				} else {
					response.IsSuccess = false
					response.Message = "REDIS not Available!"
				}

			} else {
				//Get ID and Return
				if CheckRedisAvailability(request) {
					id := keygenerator.GetIncrementID(request, "ELASTIC", 0)
					response.Body = []byte(id)
					response.IsSuccess = true
					response.Message = "Successfully Completed!"
				} else {
					response.IsSuccess = false
					response.Message = "REDIS not Available!"
				}
			}
		case "readid":
			if IsPattern {
				//pattern code goes here
				prefix, valueInString := keygenerator.GetPatternAttributes(request)
				var value int
				value, _ = strconv.Atoi(valueInString)

				if CheckRedisAvailability(request) {
					id := keygenerator.GetTentativeID(request, "ELASTIC", value)

					for x := 0; x < len(request.Controls.Class); x++ {
						if (len(prefix) + len(id)) < len(request.Controls.Class) {
							prefix += "0"
						} else {
							break
						}
					}

					id = prefix + id
					response.Body = []byte(id)
					response.IsSuccess = true
					response.Message = "Successfully Completed!"
				} else {
					response.IsSuccess = false
					response.Message = "REDIS not Available!"
				}

			} else {
				//Get ID and Return
				if CheckRedisAvailability(request) {
					id := keygenerator.GetTentativeID(request, "ELASTIC", 0)
					response.Body = []byte(id)
					response.IsSuccess = true
					response.Message = "Successfully Completed!"
				} else {
					response.IsSuccess = false
					response.Message = "REDIS not Available!"
				}
			}
		}
	default:
		return repository.search(request, request.Body.Special.Parameters)

	}

	return response

}

func (repository ElasticRepository) Test(request *messaging.ObjectRequest) {

}

//SUB FUNCTIONS
//Functions from SPECIAL and QUERY

func (repository ElasticRepository) executeQuery(request *messaging.ObjectRequest) (returnByte []byte) {
	conn := repository.getConnection(request)

	var query string

	parameters := make(map[string]interface{})

	if request.Extras["skip"] != nil {
		parameters["skip"] = request.Extras["skip"].(string)
	} else {
		parameters["skip"] = ""
	}

	if request.Extras["take"] != nil {
		parameters["take"] = request.Extras["take"].(string)
	} else {
		parameters["take"] = ""
	}

	query, err := queryparser.GetElasticQuery(request.Body.Query.Parameters, request.Controls.Namespace, request.Controls.Class, parameters)

	if err != nil {
		returnByte = getEmptyByteObject()
		return
	}

	data, err := conn.Search(request.Controls.Namespace, request.Controls.Class, nil, query)

	if err != nil {
		returnByte = getEmptyByteObject()
		return
	} else {
		var allMaps []map[string]interface{}
		allMaps = make([]map[string]interface{}, data.Hits.Len())
		for index, hit := range data.Hits.Hits {
			var currentMap map[string]interface{}
			currentMap = make(map[string]interface{})
			byteData, _ := hit.Source.MarshalJSON()
			json.Unmarshal(byteData, &currentMap)
			delete(currentMap, "__osHeaders")
			// if currentMap["workActionID"] != nil {
			// 	currentMap["workActionID"] = hit.Id
			// }
			allMaps[index] = currentMap
		}

		returnByte, _ = json.Marshal(allMaps)
	}

	return returnByte
}

func (repository ElasticRepository) executeGetFields(request *messaging.ObjectRequest) (returnByte []byte) {

	conn := repository.getConnection(request)

	query := "{\"from\": 0, \"size\": 1,\"query\":{\"query_string\" : {\"query\" : \"" + "*" + "\"}}}"

	data, err := conn.Search(request.Controls.Namespace, request.Controls.Class, nil, query)

	if err != nil {
		returnByte = getEmptyByteObject()
	} else {
		var allMaps []map[string]interface{}
		allMaps = make([]map[string]interface{}, data.Hits.Len())

		for index, hit := range data.Hits.Hits {
			var currentMap map[string]interface{}
			currentMap = make(map[string]interface{})
			byteData, _ := hit.Source.MarshalJSON()
			json.Unmarshal(byteData, &currentMap)
			allMaps[index] = currentMap
		}
		//create array to store
		var fieldList []string
		//store fields in array
		for key, _ := range allMaps[0] {
			if key != "__osHeaders" {
				fieldList = append(fieldList, key)
			}
		}
		returnByte, _ = json.Marshal(fieldList)
	}

	return
}

func (repository ElasticRepository) executeGetClasses(request *messaging.ObjectRequest) (returnByte []byte) {
	host := request.Configuration.ServerConfiguration["ELASTIC"]["Host"]
	port := request.Configuration.ServerConfiguration["ELASTIC"]["Port"]
	returnByte = repository.getByCURL(host, port, (request.Controls.Namespace + "/_mapping"))
	var mainMap map[string]interface{}
	mainMap = make(map[string]interface{})
	_ = json.Unmarshal(returnByte, &mainMap)
	var retArray []string
	//range through namespaces
	for _, index := range mainMap {
		for feature, typeDef := range index.(map[string]interface{}) {
			//if feature is MAPPING
			if feature == "mappings" {
				for typeName, _ := range typeDef.(map[string]interface{}) {
					retArray = append(retArray, typeName)
				}
			}
		}
	}
	returnByte, _ = json.Marshal(retArray)
	return
}

func (repository ElasticRepository) executeGetNamespaces(request *messaging.ObjectRequest) (returnByte []byte) {
	host := request.Configuration.ServerConfiguration["ELASTIC"]["Host"]
	port := request.Configuration.ServerConfiguration["ELASTIC"]["Port"]
	returnByte = repository.getByCURL(host, port, ("_mapping"))
	var mainMap map[string]interface{}
	mainMap = make(map[string]interface{})
	_ = json.Unmarshal(returnByte, &mainMap)
	var retArray []string
	//range through namespaces
	for index, _ := range mainMap {
		retArray = append(retArray, index)
	}
	returnByte, _ = json.Marshal(retArray)
	return
}

func (repository ElasticRepository) executeGetSelectedFields(request *messaging.ObjectRequest) (returnByte []byte) {

	skip := "0"

	if request.Extras["skip"] != nil {
		skip = request.Extras["skip"].(string)
	}

	take := "100"

	if request.Extras["take"] != nil {
		take = request.Extras["take"].(string)
	}
	conn := repository.getConnection(request)

	fieldNames := strings.Split(request.Body.Special.Parameters, " ")

	fieldString := "\"" + fieldNames[0] + "\""

	for index := 1; index < len(fieldNames); index++ {
		fieldString += "," + "\"" + fieldNames[index] + "\""
	}

	query := "{\"_source\":[" + fieldString + "],\"from\": " + skip + ", \"size\": " + take + ", \"query\":{\"query_string\" : {\"query\" : \"" + "*" + "\"}}}"

	//term.Write(query, term.Debug)

	data, err := conn.Search(request.Controls.Namespace, request.Controls.Class, nil, query)

	if err != nil {
	} else {
		var allMaps []map[string]interface{}
		allMaps = make([]map[string]interface{}, data.Hits.Len())

		for index, hit := range data.Hits.Hits {
			var currentMap map[string]interface{}
			currentMap = make(map[string]interface{})
			byteData, _ := hit.Source.MarshalJSON()
			json.Unmarshal(byteData, &currentMap)
			// if currentMap["workActionID"] != nil {
			// 	currentMap["workActionID"] = hit.Id
			// }
			allMaps[index] = currentMap
		}

		returnByte, _ = json.Marshal(allMaps)

	}

	return
}

func (repository ElasticRepository) executeDropClass(request *messaging.ObjectRequest) (err error) {
	host := request.Configuration.ServerConfiguration["ELASTIC"]["Host"]
	port := request.Configuration.ServerConfiguration["ELASTIC"]["Port"]
	url := "http://" + host + ":" + port + "/" + request.Controls.Namespace + "/" + request.Controls.Class
	req, err := http.NewRequest("DELETE", url, nil)
	client := &http.Client{}
	_, err = client.Do(req)
	return
}

func (repository ElasticRepository) executeDropNamespace(request *messaging.ObjectRequest) (err error) {
	host := request.Configuration.ServerConfiguration["ELASTIC"]["Host"]
	port := request.Configuration.ServerConfiguration["ELASTIC"]["Port"]
	url := "http://" + host + ":" + port + "/" + request.Controls.Namespace
	req, err := http.NewRequest("DELETE", url, nil)
	client := &http.Client{}
	_, err = client.Do(req)
	return
}

// Helper Functions

func (repository ElasticRepository) getByCURL(host string, port string, path string) (returnByte []byte) {
	url := "http://" + host + ":" + port + "/" + path
	req, err := http.NewRequest("GET", url, nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		//term.Write(err.Error(), 1)
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		returnByte = body
	}
	defer resp.Body.Close()
	return
}

// func (repository ElasticRepository) getConnection(request *messaging.ObjectRequest) (connection *elastigo.Conn) {
// 	connInt := connmanager.Get("ELASTIC", request.Controls.Namespace)
// 	if connInt != nil {
// 		connection = connInt.(*elastigo.Conn)
// 	} else {
// 		host := request.Configuration.ServerConfiguration["ELASTIC"]["Host"]
// 		port := request.Configuration.ServerConfiguration["ELASTIC"]["Port"]
// 		request.Log("Establishing new connection for Elastic Search " + host + ":" + port)

// 		conn := elastigo.NewConn()
// 		conn.SetHosts([]string{host})
// 		conn.Port = port
// 		connection = conn
// 		connmanager.Set("ELASTIC", request.Controls.Namespace, connection)
// 	}
// 	return
// }

var ElasticConnections map[string]*elastigo.Conn

func (repository ElasticRepository) getConnection(request *messaging.ObjectRequest) (connection *elastigo.Conn) {

	if ElasticConnections == nil {
		ElasticConnections = make(map[string]*elastigo.Conn)
	}

	host := request.Configuration.ServerConfiguration["ELASTIC"]["Host"]
	port := request.Configuration.ServerConfiguration["ELASTIC"]["Port"]

	pattern := host + ":" + port

	if ElasticConnections[pattern] != nil {
		connection = ElasticConnections[pattern]
	} else {
		conn := elastigo.NewConn()
		conn.SetHosts([]string{host})
		conn.Port = port
		connection = conn
		if ElasticConnections[pattern] != nil {
			ElasticConnections[pattern] = connection
		}
	}
	return
}

func (repository ElasticRepository) getRecordID(request *messaging.ObjectRequest, obj map[string]interface{}) (returnID string) {
	isAutoIncrementing := false
	isRandomKeyID := false

	if (obj[request.Body.Parameters.KeyProperty].(string) == "-999") || (request.Body.Parameters.AutoIncrement == true) {
		isAutoIncrementing = true
	} else if (obj[request.Body.Parameters.KeyProperty].(string) == "-888") || (request.Body.Parameters.GUIDKey == true) {
		isRandomKeyID = true
	}

	if isRandomKeyID {
		returnID = common.GetGUID()
	} else if isAutoIncrementing {
		if CheckRedisAvailability(request) {
			returnID = keygenerator.GetIncrementID(request, "ELASTIC", 0)
		} else {
			request.Log("WARNING! : Returning GUID since REDIS not available and not concurrent safe!")
			returnID = common.GetGUID()
		}
	} else {
		return obj[request.Body.Parameters.KeyProperty].(string)
	}
	return
}

func (repository ElasticRepository) ClearCache(request *messaging.ObjectRequest) {
}
