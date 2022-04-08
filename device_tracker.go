package main

import (
    "fmt"
    "strconv"
    "strings"
    "sync"
    "math/rand"
    "net"
    "time"
    log "github.com/sirupsen/logrus"
    uj "github.com/nanoscopic/ujsonin/v2/mod"
)

type Event struct {
    action     int
    uuid       string    
}

type DeviceTracker struct {
    Config       *Config
    DevMap       map [string] *Device
    freePorts    []int
    portMin      int
    portMax      int
    process      map[string] *GenericProc
    lock         *sync.Mutex
    cf           *ControlFloor
    cfStop       chan bool
    bridge       BridgeRoot
    currentDevs  []BridgeDev
    shuttingDown bool
    // only activate the specific list of ids
    idList       []string
}

func NewDeviceTracker( config *Config, detect bool, idList []string ) (*DeviceTracker) {
    var cf *ControlFloor
    var cfStop chan bool
    if detect {
        cf, cfStop = NewControlFloor( config )
    }
    
    portRange := config.portRange
    parts := strings.Split(portRange,"-")
    portMin, _ := strconv.Atoi( parts[0] )
    portMax, _ := strconv.Atoi( parts[1] )
    
    self := &DeviceTracker{
        process: make( map[string] *GenericProc ),
        lock: &sync.Mutex{},
        DevMap: make( map [string] *Device ),
        Config: config,
        portMin: portMin,
        portMax: portMax,
        freePorts: []int{},
        cf: cf,
        cfStop: cfStop,
        idList: idList,
        currentDevs: []BridgeDev{},
    }
    if detect {
        cf.DevTracker = self
    }
    
    bridgeCreator := NewIIFBridge
    bridgeCli := config.iosIfPath
    if config.bridge == "go-ios" {
        bridgeCreator = NewGIBridge
        bridgeCli = config.goIosPath
    }
    
    self.bridge = bridgeCreator(
        config,
        func( dev BridgeDev ) ProcTracker { return self.onDeviceConnect1( dev ) },
        func( dev BridgeDev ) { self.onDeviceDisconnect( dev ) },
        bridgeCli,
        self,
        detect,
    )
    return self
}

func ( self *DeviceTracker ) isShuttingDown() bool {
    return self.shuttingDown;
}

func (self *DeviceTracker) startProc( proc *GenericProc ) {
    self.lock.Lock()
    self.process[ proc.name ] = proc
    self.lock.Unlock()
}

func ( self *DeviceTracker ) stopProc( procName string ) {
    self.lock.Lock()
    delete( self.process, procName )
    self.lock.Unlock()
}

func IsFree(port string) bool {
	timeout := 1 * time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", port), timeout)
	if err != nil {
		return true
	}
	if conn != nil {
		defer conn.Close()
		return false
	}
	return true
}

func (self *DeviceTracker) getFreePort() int {
	fmt.Printf("self.portMin %d, self.portMax %d\n", self.portMin, self.portMax)
	port := rand.Intn(self.portMax-self.portMin) + self.portMin
	portnumber := strconv.Itoa(port)
	free := IsFree(portnumber)
	if free {
		fmt.Printf("Free port found : %d\n", port)
		return port
	} else {
		return self.getFreePort()
	}
}
func (self *DeviceTracker) getPort() (int) {
    var port int
    self.lock.Lock()
    if len( self.freePorts ) > 0 {
      port = self.freePorts[0]
      self.freePorts = self.freePorts[1:]
    } else {
      port = self.portMin
      self.portMin++
    }
    self.lock.Unlock()
    return port
}

func (self *DeviceTracker) freePort( port int ) {
    self.lock.Lock()
    self.freePorts = append( self.freePorts, port )
    self.lock.Unlock()
}

func (self *DeviceTracker) getDevice( udid string ) (*Device) {
    return self.DevMap[ udid ]
}

func (self *DeviceTracker) cfReady() {
    fmt.Println("Notifying CF of devices:")
    for _, bdev := range self.currentDevs {
        fmt.Printf("Device - udid: %s\n", bdev.getUdid() )
        self.onDeviceConnect1( bdev )
    }
    //self.pendingDevs = []BridgeDev{}
}

func (self *DeviceTracker) onDeviceConnect1( bdev BridgeDev ) *Device {
    udid := bdev.getUdid()
    
    startedDevice, isStarted := self.DevMap[ udid ]
    if isStarted {
        width := startedDevice.vidWidth
        height := startedDevice.vidHeight
        clickWidth := startedDevice.devConfig.uiWidth
        clickHeight := startedDevice.devConfig.uiHeight
        self.cf.notifyDeviceExists( udid, width, height, clickWidth, clickHeight )
        self.cf.notifyDeviceInfo( startedDevice, nil )
        return startedDevice
    }
    
    if len( self.idList ) > 0 {
        devFound := false
        for _,oneId := range( self.idList ) {
            if oneId == udid {
                devFound = true
            }
        }
        if !devFound { return nil }
    }
    
    found := false
    for _,adev := range self.currentDevs {
        if adev == bdev { found = true }
    }
    if !found {
        self.currentDevs = append( self.currentDevs, bdev )
    }
        
    if !self.cf.ready {
        fmt.Printf("Device attached, but ControlFloor not ready.\n  udid=%s\n", udid )
        return nil
    } else {
        self.currentDevs = append( self.currentDevs, bdev )
    }
    
    //fmt.Printf("udid: %s\n", udid)
    //dev := self.DevMap[ udid ]
    
    _, devConfOk := self.Config.devs[udid]
        
    clickWidth := 0
    clickHeight := 0
    width := 0
    height := 0
    
    
    var devConf *CDevice
    if devConfOk {
      devConfOb := self.Config.devs[udid]
      devConf = &devConfOb
    } else {
      fmt.Printf("Device not found in config.devices\n")
    }
    
    mgInfo := make( map[string]uj.JNode )
    if devConfOk && devConf.uiWidth != 0 {
        devConf := self.Config.devs[ udid ]
        clickWidth = devConf.uiWidth
        clickHeight = devConf.uiHeight
        width = clickWidth
        height = clickHeight
    } else {
        mgInfo = bdev.gestaltnode( []string{
            "AvailableDisplayZoomSizes",
            "main-screen-width",
            "main-screen-height",
            "ArtworkTraits",
        } )
        width = mgInfo["main-screen-width"].Int()
        height = mgInfo["main-screen-height"].Int()
    
        sizeArr := mgInfo["AvailableDisplayZoomSizes"].Get("default") // zoomed also available
        clickWidth = sizeArr.GetAt(1).Int()
        clickHeight = sizeArr.GetAt(3).Int()
    }
        
    self.cf.notifyDeviceExists( udid, width, height, clickWidth, clickHeight )
    dev := self.onDeviceConnect2( udid, bdev )
    self.cf.notifyDeviceInfo( dev, mgInfo["ArtworkTraits"] )
    bdev.setProcTracker( self )
    dev.startup()
    return dev
}

func (self *DeviceTracker) onDeviceDisconnect( bdev BridgeDev ) {
    udid := bdev.getUdid()
    dev := self.DevMap[ udid ]
    
    dev.connected = false
    
    dev.stopEventLoop()
    dev.shutdown()
    dev.releasePorts()
    
    delete( self.DevMap, udid )
}

func (self *DeviceTracker) shutdown() {
    self.shuttingDown = true
    
    for _,dev := range self.DevMap {
        dev.shuttingDown = true
        self.cf.notifyProvisionStopped( dev.udid )
    }
    
    for _,dev := range self.DevMap {
        log.WithFields( log.Fields{
            "type": "shutdown_device",
            "uuid": censorUuid( dev.udid ),
        } ).Info("Shutdown device")
        dev.shutdown()
    }
    
    for _,proc := range self.process {
        log.WithFields( log.Fields{
            "type": "shutdown_proc",
            "proc": proc.name,
            "pid":  proc.pid,
        } ).Info("Shutting down " + proc.name + " devproc")
        go func() { proc.Kill() }()
    }
    
    go func() { self.cfStop <- true }()
}

func (self *DeviceTracker) onDeviceConnect2( uuid string, bdev BridgeDev ) (*Device){
    log.WithFields( log.Fields{
        "type": "dev_present",
        "uuid": censorUuid( uuid ),
    } ).Info("Device Present")
    
    dev := self.DevMap[ uuid ]
    if dev != nil {
        dev.connected = true
        return dev
    }
    dev = NewDevice( self.Config, self, uuid, bdev )
    bdev.SetDevice( dev )
    
    devInfo := getAllDeviceInfo( bdev )
    
    log.WithFields( log.Fields{
        "type": "dev_info_full",
        "uuid": censorUuid( uuid ),
        "info": devInfo,
    } ).Debug("Device Info")
    
    log.WithFields( log.Fields{
        "type": "dev_info_basic",
        "uuid": censorUuid( uuid ),
        "ModelNumber": devInfo["ModelNumber"],
        "ProductType": devInfo["ProductType"],
        "ProductVersion": devInfo["ProductVersion"],
    } ).Info("Device Info")
    
    dev.info = devInfo
    dev.iosVersion = devInfo["ProductVersion"]
    versionParts := strings.Split( dev.iosVersion, "." )
    
    majorStr := versionParts[0]
    dev.versionParts[0],_ = strconv.Atoi( majorStr )
    
    if len( versionParts ) > 1 {
        medStr := versionParts[1]
        dev.versionParts[1],_ = strconv.Atoi( medStr )
    }
    
    if len( versionParts ) > 2 {
        minStr := versionParts[2]
        dev.versionParts[2],_ = strconv.Atoi( minStr )
    }
    
    self.DevMap[ uuid ] = dev
    return dev
}