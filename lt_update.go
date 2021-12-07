package main

import "encoding/json"

//MARK:-  LT Changes ==========Start==========
type CFR_Refresh struct {
	Id      int    `json:"id"`
	Refresh string `json:"refresh"`
}

func (self *CFR_Refresh) asText() string {
	text, _ := json.Marshal(self)
	return string(text)
}

type CFR_Restart struct {
	Id      int    `json:"id"`
	Restart string `json:"restart"`
}

func (self *CFR_Restart) asText() string {
	text, _ := json.Marshal(self)
	return string(text)
}

//LT Changes ==========End==========
