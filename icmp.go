package main

//Icmp by yaodinglin
/*
Usage: icmp [flags] [expr] host
  -c int
        count of ping (default 5)
  -f string
        write log to the special file
  -i int
        interval between two ping(ms) (default 1000)
  -p int
        length of padding
  -w int
        wait to time out(ms) (default 1000)

*/

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

//Main configure option of ping
func main() {
	service := os.Args[len(os.Args)-1]

	count := flag.Int("c", 5, "count of ping")
	pad := flag.Int("p", 0, "length of padding")
	interval := flag.Int("i", 1000, "interval between two ping(ms)")
	deadtime := flag.Int("w", 1000, "wait to time out(ms)")
	fileName := flag.String("f", "", "write log to the special file")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: icmp [flags] [expr] host\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	var fileS *os.File
	var err error
	if *fileName != "" {
		fileS, err = os.OpenFile(*fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	defer fileS.Close()

	cancel := make([]chan bool, *count)
	_, err = fmt.Fprintf(fileS, "Ping start at %v\n", time.Now())
	for i := 1; i <= *count; i++ {
		cancel[i-1] = make(chan bool, 1)
		go ping(service, i, *deadtime, cancel, *pad, fileS)
		time.Sleep(time.Duration(*interval) * time.Millisecond)
	}

	for i := 0; i < *count; i++ {
		defer close(cancel[i])
	}
	//fmt.Printf("Summary:send %v,lost%%%d",*count,canCount)

	os.Exit(0)
}

func ping(host string, seq int, deadtime int, cancel []chan bool, pad int, fileName *os.File) {
	seq = seq - 1
	readOk := make(chan bool, 1)
	defer close(readOk)
	timer := time.NewTicker(time.Duration(deadtime) * time.Millisecond)
	go func(host string, readOk chan bool, cancel []chan bool, seq int, pad int) {
		conn, err := net.Dial("ip4:icmp", host)
		defer conn.Close()
		checkError(err)
		msg := make([]byte, 28+pad)
		msg[0] = 8         // echo
		msg[1] = 0         // code 0
		msg[2] = 0         // checksum
		msg[3] = 0         // checksum
		msg[4] = 0         // identifier[0]
		msg[5] = 13        //identifier[1]
		msg[6] = 0         // sequence[0]
		msg[7] = byte(seq) // sequence[1]
		len := 8 + pad
		if pad != 0 {
			for i := 8; i < 8+pad; i++ {
				msg = append(msg, byte((i+256-7)%256))
			}
		}
		check := checkSum(msg[0:len])
		msg[2] = byte(check >> 8)
		msg[3] = byte(check & 255)
		timeOut := time.Now()
		_, err = conn.Write(msg[0:len])

		checkError(err)
		_, err = conn.Read(msg[0:])
		timeIn := time.Now()
		checkError(err)
		select {
		case <-cancel[seq]:
			cancel[seq] <- true
			return
		default:
			readOk <- true
		}
		_, err = fmt.Fprintf(fileName, "Seq=%v\t, Reply from %v, Spent time:%v\n", seq+1, net.IPv4(msg[12], msg[13], msg[14], msg[15]),
			timeIn.Sub(timeOut))
		fmt.Printf("Seq=%v\t, Reply from %v, Spent time:%v\n", seq+1, net.IPv4(msg[12], msg[13], msg[14], msg[15]),
			timeIn.Sub(timeOut))
		if msg[25] == 13 {
		} else {
			fmt.Fprintf(fileName, "Identifier dismatches,reply is %v\n", msg[5])
			fmt.Printf("Identifier dismatches,reply is %v\n", msg[5])
		}
		if msg[27] == byte(seq) {
		} else {
			fmt.Fprintf(fileName, "Sequence dismatches,reply is %v\n", msg[7])
			fmt.Printf("Sequence dismatches,reply is %v\n", msg[7])
		}
		check1 := uint16(msg[22])*256 + uint16(msg[23])
		msg[22] = 0
		msg[23] = 0
		check2 := checkSum(msg[20:28])
		if check2 != check1 {
			fmt.Fprintf(fileName, "CheckSum dismatches,reply is %v,it should be %v\n", check1, check2)
			fmt.Printf("CheckSum dismatches,reply is %v,it should be %v\n", check1, check2)
		}

	}(host, readOk, cancel, seq, pad)

	select {
	case <-readOk:
		return
	case <-timer.C:
		cancel[seq] <- true
		fmt.Fprintf(fileName, "Time out for seq %v\n", seq+1)
		fmt.Printf("Time out for seq %v\n", seq+1)
		return

	}

}

//CheckSum func check the checkSum of the given slice of byte ,return uint16
func checkSum(msg []byte) uint16 {
	sum := 0
	length := len(msg)
	if length%2 == 1 {
		msg = append(msg, 0)
	}
	for n := 0; n < len(msg)-1; n += 2 {
		sum += int(msg[n])*256 + int(msg[n+1])
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum += (sum >> 16)
	var answer uint16 = uint16(^sum)
	return answer
}
func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s\n", err.Error())
		os.Exit(1)
	}
}
