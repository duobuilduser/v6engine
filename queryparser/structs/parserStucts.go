package structs

type QueryObject struct {
	Operation      string            //select, show, describe, explain
	SelectedFields []string          //["Name", "Age"]
	Table          string            //student
	Where          map[int][]string  //map[0]["age", ">=", "24"]
	Orderby        map[string]string //map["Name"] = "ASC"
}

type RepoRequest struct {
	Repository  string
	Query       string
	Queryobject QueryObject
	Parameters  map[string]interface{}
}

type RepoResponse struct {
	Query interface{}
	Err   error
}
