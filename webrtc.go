package main

import (
    "fmt"
    "encoding/json"
    "encoding/base64"
    "github.com/pion/webrtc/v3"
)

// Encode encodes the input in base64
func UtilEncode(obj interface{}) string {
    b, err := json.Marshal(obj)
    if err != nil {
        panic(err)
    }

    return base64.StdEncoding.EncodeToString(b)
}

// Decode decodes the input from base64
func UtilDecode(in string, obj interface{}) {
    b, err := base64.StdEncoding.DecodeString(in)
    if err != nil {
        panic(err)
    }

    err = json.Unmarshal(b, obj)
    if err != nil {
        panic(err)
    }
}


func startWebRtc( 
    offerBase64 string,
    onMsg func(msg webrtc.DataChannelMessage),
    onOpen func(d *webrtc.DataChannel),
    onExit func(),
) (*webrtc.PeerConnection, string) {
    // Prepare the configuration
    config := webrtc.Configuration{
        ICEServers: []webrtc.ICEServer{
            {
                URLs: []string{"stun:dryark.com:3478"},
                //URLs: []string{"stun:stun.l.google.com:19302"},
            },
        },
    }

    // Create a new RTCPeerConnection
    peerConnection, err := webrtc.NewPeerConnection(config)
    if err != nil {
        panic(err)
    }
    /*defer func() {
        if cErr := peerConnection.Close(); cErr != nil {
            fmt.Printf("cannot close peerConnection: %v\n", cErr)
        }
    }()*/

    // Set the handler for Peer connection state
    // This will notify you when the peer has connected/disconnected
    peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
        fmt.Printf("Peer Connection State has changed: %s\n", s.String())

        if s == webrtc.PeerConnectionStateFailed {
            // Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
            // Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
            // Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
            //fmt.Println("Peer Connection has gone to failed exiting")
            //os.Exit(0)
            onExit()
        }
    })

    // Register data channel creation handling
    peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
        fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())
        d.OnOpen( func() { onOpen(d) } )
        d.OnMessage( onMsg )
    })

    offer := webrtc.SessionDescription{}
    UtilDecode(offerBase64, &offer)

    // Set the remote SessionDescription
    err = peerConnection.SetRemoteDescription(offer)
    if err != nil {
        panic(err)
    }

    // Create an answer
    answer, err := peerConnection.CreateAnswer(nil)
    if err != nil {
        panic(err)
    }

    // Create channel that is blocked until ICE Gathering is complete
    //gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

    // Sets the LocalDescription, and starts our UDP listeners
    err = peerConnection.SetLocalDescription(answer)
    if err != nil {
        panic(err)
    }

    // Block until ICE Gathering is complete, disabling trickle ICE
    // we do this because we only can exchange one signaling message
    // in a production application you should exchange ICE Candidates via OnICECandidate
    //<- gatherComplete

    // Output the answer in base64 so we can paste it in browser
    answerStr := UtilEncode(*peerConnection.LocalDescription())

    return peerConnection, answerStr
}
