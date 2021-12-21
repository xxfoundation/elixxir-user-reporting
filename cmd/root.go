package cmd

import (
	"fmt"
	"git.xx.network/elixxir/user-reporting/messages"
	"git.xx.network/elixxir/user-reporting/reports"
	"git.xx.network/elixxir/user-reporting/storage"
	"github.com/golang/protobuf/proto"
	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/elixxir/client/api"
	"gitlab.com/elixxir/client/interfaces/message"
	"gitlab.com/elixxir/client/interfaces/params"
	"gitlab.com/elixxir/crypto/contact"
	"gitlab.com/elixxir/primitives/fact"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/utils"
	"io/ioutil"
	"net"
	"os"
	"time"
)

var (
	cfgFile, logPath string
	validConfig      bool
)

// RootCmd represents the base command when called without any sub-commands
var rootCmd = &cobra.Command{
	Use:   "Reports",
	Short: "Runs the cMix reports bot.",
	Long:  "The reports bot accepts messenger connections for user reports.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize config & logging
		initConfig()
		initLog()

		// Get database parameters
		rawAddr := viper.GetString("dbAddress")
		var addr, port string
		var err error
		if rawAddr != "" {
			addr, port, err = net.SplitHostPort(rawAddr)
			if err != nil {
				jww.FATAL.Panicf("Unable to get database port from %s: %+v", rawAddr, err)
			}
		}

		sp := storage.Params{
			Username: viper.GetString("dbUsername"),
			Password: viper.GetString("dbPassword"),
			DBName:   viper.GetString("dbName"),
			Address:  addr,
			Port:     port,
		}

		// Initialize storage object
		s, err := storage.NewStorage(sp)
		if err != nil {
			jww.FATAL.Panicf("Failed to initialize storage interface: %+v", err)
		}

		// Get session parameters
		sessionPath := viper.GetString("sessionPath")
		sessionPass := viper.GetString("sessionPass")
		networkFollowerTimeout := time.Duration(viper.GetInt("networkFollowerTimeout")) * time.Second

		// Create the client if there's no session
		if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
			ndfPath := viper.GetString("ndf")
			ndfJSON, err := ioutil.ReadFile(ndfPath)
			if err != nil {
				jww.FATAL.Panicf("Failed to read NDF: %+v", err)
			}
			err = api.NewClient(string(ndfJSON), sessionPath, []byte(sessionPass), "")
			if err != nil {
				jww.FATAL.Panicf("Failed to create new client: %+v", err)
			}
		}

		// Create client object
		cl, err := api.Login(sessionPath, []byte(sessionPass), params.GetDefaultNetwork())
		if err != nil {
			jww.FATAL.Panicf("Failed to initialize client: %+v", err)
		}

		// Generate QR code
		qrSize := viper.GetInt("qrSize")
		qrLevel := qrcode.RecoveryLevel(viper.GetInt("qrLevel"))
		qrPath := viper.GetString("qrPath")
		me := cl.GetUser().GetContact()
		reportsUname, err := fact.NewFact(fact.Username, "xx user reports bot")
		if err != nil {
			jww.FATAL.Printf("Failed to create reports username fact: %+v", err)
		}
		me.Facts = append(me.Facts, reportsUname)
		qr, err := me.MakeQR(qrSize, qrLevel)
		if err != nil {
			jww.FATAL.Panicf("Failed to generate QR code: %+v", err)
		}
		// Save the QR code PNG to file
		err = utils.WriteFile(qrPath, qr, utils.FilePerms, utils.DirPerms)
		if err != nil {
			jww.FATAL.Panicf("Failed to write QR code: %+v", err)
		}

		// Start network follower
		err = cl.StartNetworkFollower(networkFollowerTimeout)
		if err != nil {
			jww.FATAL.Panicf("Failed to start network follower: %+v", err)
		}

		// Create & register callback to confirm any authenticated channel requests
		rcb := func(requestor contact.Contact, msg string) {
			rid, err := cl.ConfirmAuthenticatedChannel(requestor)
			if err != nil {
				jww.ERROR.Printf("Failed to confirm authenticated channel to %+v: %+v", requestor, err)
				return
			}
			jww.DEBUG.Printf("Authenticated channel to %+v created over round %d", requestor, rid)

			// WAIT 15 SECONDS THEN SAY HELLO
			time.Sleep(15000 * time.Millisecond)

			intro := "Hi!  I'm the User Reporting bot."
			payload := &messages.CMIXText{Text: intro}
			marshalled, err := proto.Marshal(payload)
			if err != nil {
				jww.ERROR.Printf("Failed to marshal payload: %+v", err)
				return
			}

			contact, err := cl.GetAuthenticatedChannelRequest(requestor.ID)
			if err != nil {
				jww.ERROR.Printf("Could not get authenticated channel request info: %+v", err)
				return
			}

			// Create response message
			resp := message.Send{
				Recipient:   contact.ID,
				Payload:     marshalled,
				MessageType: message.Text,
			}

			rids, mid, t, err := cl.SendE2E(resp, params.GetDefaultE2E())
			if err != nil {
				jww.ERROR.Printf("Failed to send message: %+v", err)
				return
			}
			jww.INFO.Printf("Sent intro [%+v] to %+v on rounds %+v [%+v]", mid, requestor, rids, t)
		}
		cl.GetAuthRegistrar().AddGeneralRequestCallback(rcb)

		// Create impl & register listener on zero user for text messages
		impl := reports.New(s, cl)
		cl.GetSwitchboard().RegisterListener(&id.ZeroUser, message.Text, impl)

		// Wait 5ever
		select {}
	},
}

// Execute calls the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		jww.ERROR.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "",
		"Path to load the UDB configuration file from. If not set, this "+
			"file must be named udb.yaml and must be located in "+
			"~/.xxnetwork/, /opt/xxnetwork, or /etc/xxnetwork.")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	validConfig = true
	var err error
	if cfgFile == "" {
		cfgFile, err = utils.SearchDefaultLocations("reports.yaml", "xxnetwork")
		if err != nil {
			validConfig = false
			jww.FATAL.Panicf("Failed to find config file: %+v", err)
		}
	} else {
		cfgFile, err = utils.ExpandPath(cfgFile)
		if err != nil {
			validConfig = false
			jww.FATAL.Panicf("Failed to expand config file path: %+v", err)
		}
	}
	viper.SetConfigFile(cfgFile)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Unable to read config file (%s): %+v", cfgFile, err.Error())
		validConfig = false
	}
}

// initLog initializes logging thresholds and the log path.
func initLog() {
	vipLogLevel := viper.GetUint("logLevel")

	// Check the level of logs to display
	if vipLogLevel > 1 {
		// Set the GRPC log level
		err := os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "info")
		if err != nil {
			jww.ERROR.Printf("Could not set GRPC_GO_LOG_SEVERITY_LEVEL: %+v", err)
		}

		err = os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "99")
		if err != nil {
			jww.ERROR.Printf("Could not set GRPC_GO_LOG_VERBOSITY_LEVEL: %+v", err)
		}
		// Turn on trace logs
		jww.SetLogThreshold(jww.LevelTrace)
		jww.SetStdoutThreshold(jww.LevelTrace)
	} else if vipLogLevel == 1 {
		// Turn on debugging logs
		jww.SetLogThreshold(jww.LevelDebug)
		jww.SetStdoutThreshold(jww.LevelDebug)
	} else {
		// Turn on info logs
		jww.SetLogThreshold(jww.LevelInfo)
		jww.SetStdoutThreshold(jww.LevelInfo)
	}

	logPath = viper.GetString("log")

	logFile, err := os.OpenFile(logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644)
	if err != nil {
		fmt.Printf("Could not open log file %s!\n", logPath)
	} else {
		jww.SetLogOutput(logFile)
	}
}
