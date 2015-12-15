package main

type AuthEvent struct {
	Time        string // number of seconds from Unix epoch
	AuthType    string // type of event - eg sshPass, sshKey
	SrcIP       string //
	DestIP      string //
	User        string // username used
	Credentials string // ssh password or ssh key used
	TypeData    string // extra data specific to the auth type. May be json and/or base64 encoded
	Hash        string // mostly uniq hash of the event
}
