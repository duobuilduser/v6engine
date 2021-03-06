package queryparser

import (
	"duov6.com/queryparser/analyzer"
	"duov6.com/queryparser/repositories"
	"duov6.com/queryparser/structs"
	"errors"
	//"fmt"
	"google.golang.org/cloud/datastore"
	"strings"
)

//This is the main entry point to the query parser

func GetElasticQuery(queryString string, namespace string, class string, parameters map[string]interface{}) (query string, err error) {
	if queryResult, er := getQuery(queryString, "ES", namespace, class, parameters); er == nil {
		query = queryResult.(string)
	} else {
		err = er
	}
	return
}

func GetDataStoreQuery(queryString string, namespace string, class string, parameters map[string]interface{}) (query *datastore.Query, err error) {
	if queryResult, er := getQuery(queryString, "CDS", namespace, class, parameters); er == nil {
		query = queryResult.(*datastore.Query)
	} else {
		err = er
	}
	return
}

func GetMsSQLQuery(queryString string, namespace string, class string, parameters map[string]interface{}) (query string, err error) {
	if queryResult, er := getQuery(queryString, "MSSQL", namespace, class, parameters); er == nil {
		query = queryResult.(string)
	} else {
		err = er
	}
	return
}

func GetCloudSQLQuery(queryString string, namespace string, class string, parameters map[string]interface{}) (query string, err error) {
	if queryResult, er := getQuery(queryString, "CSQL", namespace, class, parameters); er == nil {
		query = queryResult.(string)
	} else {
		err = er
	}
	return
}

func GetPostgresQuery(queryString string, namespace string, class string, parameters map[string]interface{}) (query string, err error) {
	if queryResult, er := getQuery(queryString, "PSQL", namespace, class, parameters); er == nil {
		query = queryResult.(string)
	} else {
		err = er
	}
	return
}

func GetMySQLQuery(queryString string, namespace string, class string, parameters map[string]interface{}) (query string, err error) {
	if queryResult, er := getQuery(queryString, "MYSQL", namespace, class, parameters); er == nil {
		query = queryResult.(string)
	} else {
		err = er
	}
	return
}

func GetHiveQuery(queryString string, namespace string, class string, parameters map[string]interface{}) (query string, err error) {
	if queryResult, er := getQuery(queryString, "HSQL", namespace, class, parameters); er == nil {
		query = queryResult.(string)
	} else {
		err = er
	}
	return
}

func getQuery(queryString string, repository string, namespace string, class string, parameters map[string]interface{}) (queryResult interface{}, err error) {

	queryTokens := strings.Split(strings.ToLower(queryString), " ")

	if queryTokens[0] != "select" {
		err = errors.New("Invalid Query! Only SELECT statements are allowed!")
		return
	}

	//get type of query
	if queryType := analyzer.GetQueryType(queryString); queryType == "SQL" {
		//fmt.Println("SQL Query Identified!")
		//Check is valid for preprocessing. Create normalized query
		preparedQuery, err := analyzer.PrepareSQLStatement(queryString, repository, namespace, class)
		if err != nil {
			return queryResult, err
		}

		//Create Query map from the normalized query
		queryStruct := analyzer.GetQueryMaps(preparedQuery)
		// fmt.Println("------------------")
		// fmt.Println(queryStruct.Operation)
		// fmt.Println(queryStruct.SelectedFields)
		// fmt.Println(queryStruct.Table)
		// fmt.Println(queryStruct.Where)
		// fmt.Println(queryStruct.Orderby)
		// fmt.Println("------------------")
		//check for query Compatibility
		compErr := analyzer.CheckQueryCompatibility(preparedQuery, repository, queryStruct)
		if compErr != nil {
			return "error", compErr
		}

		//Do secondary validation.. for sql keywords
		err = analyzer.ValidateQuery(queryStruct)
		if err != nil {
			return "error", err
		}

		queryRequest := structs.RepoRequest{}
		queryRequest.Repository = repository
		queryRequest.Query = preparedQuery
		queryRequest.Queryobject = queryStruct
		queryRequest.Parameters = make(map[string]interface{})
		queryRequest.Parameters = parameters

		response := repositories.Execute(queryRequest)
		if response.Err != nil {
			err = response.Err
			return response.Query, err
		}

		queryResult = response.Query

	} else {
		//reply other query
		//fmt.Println("OTHER")
		queryResult = analyzer.GetOtherQuery(queryString, repository)
	}
	return
}
