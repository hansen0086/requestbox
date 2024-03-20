package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "request_box_gauge",
			Help: "Total number of requests.",
		},
		[]string{"token"},
	)
)

func init() {
	prometheus.MustRegister(requestCount)
}

type Request struct {
	Method string              `json:"Method"`
	URL    string              `json:"Url"`
	Header map[string][]string `json:"Header"`
	Body   interface{}         `json:"Body"`
	Time   int64               `json:"Time"`
}

type Users struct {
	Users []User `json:"users"`
}

type User struct {
	Token           string       `json:"token"`
	UserRequestList *RequestList `json:"-"`
}

type RequestList struct {
	requests []*Request
	lock     sync.RWMutex
}

func GetUserByToken(token string, users Users) (*User, error) {
	if token == "" {
		return nil, errors.New("token is empty")
	}
	for _, user := range users.Users {
		if user.Token == token {
			return &user, nil
		}
	}
	return nil, errors.New("user not found")
}

func PrintUsers(users Users) {
	for _, user := range users.Users {
		PrintUser(user)
	}
}

func PrintUser(user User) {
	log.Println("printing User Token: ", user.Token)
	for _, req := range user.UserRequestList.List() {
		log.Println("Request: ", req.Body)
	}
}

func ListUsersString(users Users) string {
	var list string
	for _, user := range users.Users {
		list += user.Token + "\n"
	}
	return list
}

func ListUsersJson(users Users) []byte {
	json, _ := json.Marshal(users)
	return json
}
func (rl *RequestList) Add(req *Request) {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	//prepend to front
	rl.requests = append([]*Request{req}, rl.requests...)

	if len(rl.requests) >= 10 {
		//get first 10 elements
		rl.requests = rl.requests[:10]
	}
}

func (rl *RequestList) List() []*Request {
	rl.lock.RLock()
	defer rl.lock.RUnlock()

	return rl.requests
}

func (rl *RequestList) Len() int {
	rl.lock.RLock()
	defer rl.lock.RUnlock()

	return len(rl.requests)
}

func (rl *RequestList) Less(i, j int) bool {
	rl.lock.RLock()
	defer rl.lock.RUnlock()

	return rl.requests[i].Time > rl.requests[j].Time
}

func (rl *RequestList) Swap(i, j int) {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	rl.requests[i], rl.requests[j] = rl.requests[j], rl.requests[i]
}

func main() {
	//create Users
	users := &Users{
		Users: []User{},
	}

	//Create Token
	//Create User and set uuid for it
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/CreateToken", func(w http.ResponseWriter, r *http.Request) {
		// 读取请求的 Body
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		id := uuid.New()
		user := User{
			Token: id.String(),
			UserRequestList: &RequestList{
				requests: make([]*Request, 0),
				lock:     sync.RWMutex{},
			},
		}
		users.Users = append(users.Users, user)

		usersString := ListUsersString(*users)
		// Initialize token with count 0
		requestCount.With(prometheus.Labels{"token": id.String()}).Add(0)

		log.Printf("Current Tokens:\n%s", usersString)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//write token to response
		resp := make(map[string]string)
		resp["token"] = id.String()
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Fatalf("Error happened in JSON marshal. Err: %s", err)
		}
		w.Write(jsonResp)
	})
	http.HandleFunc("/DeleteToken", func(w http.ResponseWriter, r *http.Request) {
		// 读取请求的 Body
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Token is empty", http.StatusBadRequest)
			return
		}

		_, err = GetUserByToken(token, *users)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		for i, user := range users.Users {
			if user.Token == token {
				users.Users = append(users.Users[:i], users.Users[i+1:]...)
			}
		}
		requestCount.Delete(prometheus.Labels{"token": token})
		usersString := ListUsersString(*users)
		log.Printf("Current Tokens : %s\n", usersString)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := make(map[string]string)
		resp["token"] = token
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Fatalf("Error happened in JSON marshal. Err: %s", err)
		}
		w.Write(jsonResp)
	})
	http.HandleFunc("/ListToken", func(w http.ResponseWriter, r *http.Request) {
		// 读取请求的 Body
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		//usersString := ListUsersString(*users)
		//log.Printf("Current Tokens : %s", usersString)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := ListUsersJson(*users)
		//resp["token"] = usersString
		w.Write(resp)

	})

	// 创建一个用于记录请求的 RequestList
	requestList := &RequestList{}
	http.HandleFunc("/PostRequest", func(w http.ResponseWriter, r *http.Request) {
		// 读取请求的 Body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// Determine the type of the request body based on the Content-Type
		var requestBody interface{}
		contentType := r.Header.Get("Content-Type")
		if contentType == "application/json" {
			// Unmarshal the JSON body into a map or struct
			var jsonBody interface{}
			err := json.Unmarshal(body, &jsonBody)
			if err != nil {
				http.Error(w, "Failed to unmarshal JSON body", http.StatusBadRequest)
				return
			}
			requestBody = jsonBody
		} else {
			// Keep the body as a string
			requestBody = string(body)
		}

		//获取token
		token := r.URL.Query().Get("token")
		//获取user
		user, err := GetUserByToken(token, *users)
		if err != nil {
			http.Error(w, "token not found", http.StatusNotFound)
			return
		}

		// 记录请求
		request := &Request{
			Method: r.Method,
			URL:    r.URL.String(),
			Header: r.Header,
			Body:   requestBody,
			Time:   time.Now().Unix(),
		}
		user.UserRequestList.Add(request)
		req, err := json.Marshal(request)
		if err != nil {
			return
		}
		log.Printf("Request caught:\n%s\n", req)

		requestCount.With(prometheus.Labels{"token": token}).Inc()

		// 返回 200
		w.WriteHeader(http.StatusOK)
	})

	// 启动第二个端口，返回最近的 10 条请求
	http.HandleFunc("/GetRequest", func(w http.ResponseWriter, r *http.Request) {
		//获取token
		token := r.URL.Query().Get("token")
		//获取user
		user, err := GetUserByToken(token, *users)
		if err != nil {
			http.Error(w, "token not found", http.StatusNotFound)
			return
		}
		// 获取请求列表并按照时间降序排序
		requests := user.UserRequestList.List()
		sort.Sort(requestList)

		// 取出最近的 10 条请求并序列化为 JSON
		numRequests := len(requests)
		if numRequests > 10 {
			numRequests = 10
		}
		jsonBytes, err := json.Marshal(requests[:numRequests])
		if err != nil {
			http.Error(w, "Failed to encode response as JSON", http.StatusInternalServerError)
			return
		}
		// 返回 JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes)
	})
	http.HandleFunc("/CleanRequest", func(w http.ResponseWriter, r *http.Request) {
		//获取token
		token := r.URL.Query().Get("token")
		//获取user
		user, err := GetUserByToken(token, *users)
		if err != nil {
			http.Error(w, "token not found", http.StatusNotFound)
			return
		}
		// 删除token box下的全部请求
		user.UserRequestList.requests = make([]*Request, 0)
		// 删除 token下清零
		requestCount.With(prometheus.Labels{"token": token}).Set(0)

		//write token to response
		resp := make(map[string]string)
		resp["token"] = user.Token
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Fatalf("Error happened in JSON marshal. Err: %s", err)
		}
		w.Write(jsonResp)
		// 返回 JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	})
	http.ListenAndServe(":8080", nil)
}
