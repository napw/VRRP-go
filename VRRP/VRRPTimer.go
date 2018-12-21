package VRRP

import (
	"VRRP/logger"
	"time"
)

type AdvertTimer struct {
	ticker   *time.Ticker
	shutdown chan bool
	task     func()
}

type MasterDownTimer struct {
	timer    *time.Timer
	shutdown chan bool
	task     func()
}

func NewMasterDownTimer(IntervalCentSec int, Task func()) *MasterDownTimer {
	return &MasterDownTimer{timer: time.NewTimer(time.Duration(IntervalCentSec*10) * time.Millisecond), shutdown: make(chan bool), task: Task}
}

func NewAdvertTimer(IntervalCentSec int, Task func()) *AdvertTimer {
	return &AdvertTimer{ticker: time.NewTicker(time.Duration(IntervalCentSec*10) * time.Millisecond), shutdown: make(chan bool), task: Task}
}

func (ticker *AdvertTimer) Run() {
	for tik := range ticker.ticker.C {
		logger.GLoger.Printf(logger.DEBUG, "advertTimer ticked at %v", tik)
		select {
		case stop := <-ticker.shutdown:
			if stop {
				logger.GLoger.Printf(logger.DEBUG, "advertTimer stopped at %v", time.Now())
				ticker.ticker.Stop()
				ticker.shutdown <- true
				return
			}
		default:
			ticker.task()
		}
	}
}

func (timer *MasterDownTimer) Run() {
	logger.GLoger.Printf(logger.DEBUG, "master down timer started at %v", time.Now())
	select {
	case stop := <-timer.shutdown:
		if stop {
			if !timer.timer.Stop() {
				<-timer.timer.C
			}
			timer.shutdown <- true
			return
		}
	case expired := <-timer.timer.C:
		if true {
			timer.task()
			logger.GLoger.Printf(logger.DEBUG, "master down timer expired at %v", expired)

		}
	}
}

func (ticker *AdvertTimer) Stop() {
	ticker.shutdown <- true
	<-ticker.shutdown
	//todo maybe add timeout check
}

func (timer *MasterDownTimer) Stop() {
	timer.shutdown <- true
	<-timer.shutdown
}
