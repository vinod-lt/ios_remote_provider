package main

import (
    "fmt"
    "os"
    _ "github.com/mattn/go-sqlite3"
    "xorm.io/xorm"
    log "github.com/sirupsen/logrus"
)

var gDb *xorm.Engine

func openDbConnection() {
    gDb = openDb()
}

type DbApp struct {
    Udid        string `xorm:"pk"`
    Bundle      string `xorm:"pk"`
}
func (DbApp) TableName() string {
    return "app"
}

func openDb() ( *xorm.Engine ) {
    doesNotExist := false
    if _, err := os.Stat( "db.sqlite3" ); os.IsNotExist( err ) {
        doesNotExist = true
    }
    
    engine, err := xorm.NewEngine( "sqlite3", "./db.sqlite3" )
    if err != nil {
        panic( err )
    }
    
    if !doesNotExist {
        return engine
    }
            
    err = engine.Sync2( new( DbApp ) )
    if err != nil {
    }
    
    return engine
}

func getApps( udid string ) ([]string) {
    log.WithFields( log.Fields{
        "type": "app_restrict_list",
        "udid": censorUuid( udid ),
    } ).Info("Fetching app restrictions")
    
    var app DbApp
    var apps []DbApp
    err := gDb.Table(&app).Where("udid = ?", udid).Find(&apps)
    if err != nil {
        fmt.Printf("Error listing restrictions: %s\n", err )
        return []string{}
    }
    appStrs := []string{}
    for _,entry := range apps {
        appStrs = append( appStrs, entry.Bundle )
    }
    return appStrs
}

func dbRestrictApp( udid string, bundle string ) bool {
    log.WithFields( log.Fields{
        "type": "app_restrict",
        "udid": censorUuid( udid ),
        "bundle": bundle,
    } ).Info("Adding app restriction")
    
    rv := DbApp{
        Udid: udid,
        Bundle: bundle,
    }
    _, err := gDb.Insert( &rv )
    if err != nil {
        fmt.Printf("Error adding app restriction: %s\n", err )
        return false
    }
    return true
}

func dbAllowApp( udid string, bundle string ) {
    log.WithFields( log.Fields{
        "type": "app_allow",
        "udid": censorUuid( udid ),
        "bundle": bundle,
    } ).Info("Deleting device restriction")
    
    rv := DbApp{
        Udid: udid,
        Bundle: bundle,
    }
    affected, err := gDb.Where("Udid=? and Bundle=?", udid, bundle ).Delete( &rv )
    if err != nil  {
        fmt.Printf("Error: %s\n", err )
        panic("Delete reservation error")
    }
    if affected==0 {
      fmt.Printf("Delete restriction with udid %s and bid %s; no rows deleted\n", censorUuid( udid ), bundle )
    }
}