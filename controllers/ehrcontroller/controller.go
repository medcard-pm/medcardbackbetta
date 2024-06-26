package ehrcontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"medcard-new/begening/controllers/handlefile"
	"medcard-new/begening/controllers/jwtgen"
	"medcard-new/begening/controllers/velidation"
	"net/http"
	"strconv"
	"strings"

	"medcard-new/begening/evtvariables"
	"medcard-new/begening/structures"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ctx    context.Context
	client *mongo.Client
)

func Authenticationservice() {
	clientOptions := options.Client().ApplyURI(evtvariables.DBUrl)

	clientG, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Println("Mongo.connect() ERROR:2 ", err)
	}
	ctxG, _ := context.WithTimeout(context.Background(), 15*time.Minute)
	ctx = ctxG
	client = clientG
}
func DoctorClientForView(c *gin.Context) {
	var (
		ViewStruct       structures.Views
		ViewStructDecode structures.Views
		DoctorDecode     structures.SignupDoctor
		DoctorDecodeUser structures.Signup
	)
	c.ShouldBindJSON(&ViewStruct)
	CookieData := jwtgen.Velidation(c)

	stringJSON, err := json.Marshal(ViewStruct)
	if err != nil {
		log.Printf("Marshel err %v\n", err)
	}

	var dateZoneFormat string
	if ViewStruct.Date != "" {
		//? Create time zone from date and time
		splitedDate := strings.Split(ViewStruct.Date, "-")
		splitedTime := strings.Split(strings.Split(ViewStruct.Date, ";")[1], ":")
		Year, _ := strconv.Atoi(splitedDate[0])
		Month, _ := strconv.Atoi(splitedDate[1])
		Date := strings.Split(splitedDate[2], ";")[0]
		Hour, _ := strconv.Atoi(splitedTime[0])
		Minute := splitedTime[1]
		if Month > 9 {
			dateZoneFormat = fmt.Sprintf("%v-%v-%vT%v:%v:%v+%v", Year, Month, Date, Hour, Minute, "00", "05:00")
		}else if Month < 9 && Month > 0{
			dateZoneFormat = fmt.Sprintf("%v-0%v-%vT%v:%v:%v+%v", Year, Month, Date, Hour, Minute, "00", "05:00")
		}
	}

	// """"""""""""""""""""""""""""""""""DB CONNECTION""""""""""""""""""""""""""""""""""""""""""""""""""""
	Authenticationservice()
	collection := client.Database("MedCard").Collection("views")
	collectionToDoc := client.Database("MedCard").Collection("users")

	if CookieData.Permissions == "client" {
		isPassedFields, _ := velidation.TestTheStruct(c, "doctorid:sickness:clientphone", string(stringJSON), "FieldsCheck:true,DBCheck:false", "", "")

		err := collection.FindOne(ctx, bson.M{"clientid": CookieData.Id, "doctorid": ViewStruct.DoctorId}).Decode(&ViewStructDecode)
		if err != nil {
			log.Printf("Find ERR views%v\n", err)
		}
		// """""""""""""""""""""""""""""""""" Get Doctor""""""""""""""""""""""""""""""""""""""""""""""""""""
		collectionToDoc.FindOne(ctx, bson.M{"_id": ViewStruct.DoctorId}).Decode(&DoctorDecode)
		collectionToDoc.FindOne(ctx, bson.M{"_id": ViewStruct.ClientId}).Decode(&DoctorDecodeUser)
		if isPassedFields == true && ViewStructDecode.Sickness == "" && DoctorDecode.Userid != "" && DoctorDecodeUser.Userid != "" {
			// ? prepeare for inserting the data
			premetivid := primitive.NewObjectID().Hex()
			ViewStruct.Id = premetivid
			ViewStruct.DoctorPhone = DoctorDecode.Phone
			ViewStruct.ClientFLSname = fmt.Sprintf("%v %v", DoctorDecodeUser.Surname, DoctorDecodeUser.Name)
			ViewStruct.ClientId = CookieData.Id
			ViewStruct.DoctorFLSname = DoctorDecode.Surname + " " + DoctorDecode.Name
			ViewStruct.Date = ""
			_, err := collection.InsertOne(ctx, ViewStruct)
			if err != nil {
				log.Printf("Insert ERR views%v\n", err)
			}
		} else {
			c.JSON(400, gin.H{
				"Code": "You Just pusted such request",
			})
		}
	} else if CookieData.Permissions == "doctor" {
		isPassedFields, _ := velidation.TestTheStruct(c, "clientid:doctorid:date", string(stringJSON), "FieldsCheck:true,DBCheck:false", "", "")

		// """"""""""""""""""""""""""""""""""DB CONNECTION""""""""""""""""""""""""""""""""""""""""""""""""""""
		err := collection.FindOne(ctx, bson.M{"clientid": ViewStruct.ClientId, "doctorid": ViewStruct.DoctorId}).Decode(&ViewStructDecode)
		if err != nil {
			log.Printf("Find ERR views%v\n", err)
		}
		// ? Set new time and validate it if it is expired date
		now := time.FixedZone("tajikistan", 5*3600)

		dateZoneFormatParse, err := time.Parse(time.RFC3339, dateZoneFormat)
		if err != nil {
			log.Printf("Insert %v\n", err)
		}
		if ViewStructDecode.Sickness != "" && isPassedFields == true && time.Now().In(now).Before(dateZoneFormatParse) {
			collection.DeleteOne(ctx, bson.M{"clientid": ViewStruct.ClientId, "doctorid": ViewStruct.DoctorId})

			ViewStructDecode.Date = dateZoneFormat
			// ? insert into db
			_, err = collection.InsertOne(ctx, ViewStructDecode)
			if err != nil {
				log.Printf("Insert || delete Error%v\n", err)
				return
			}

			// ? Call deletion func to remove views from db
			go removeViewsFromDB(ViewStructDecode.Id)
		} else {
			if !time.Now().In(now).Before(dateZoneFormatParse) {
				c.JSON(400, gin.H{
					"Code": "Invalid date",
				})
			} else {
				c.JSON(400, gin.H{
					"Code": "Cannot Find the user",
				})
			}
		}
	} else if CookieData.Permissions == "admin" {
		isPassedFields, _ := velidation.TestTheStruct(c, "doctorid:date:clientid:sickness:clientphone", string(stringJSON), "FieldsCheck:true,DBCheck:false", "", "")
		// """"""""""""""""""""""""""""""""""DB CONNECTION""""""""""""""""""""""""""""""""""""""""""""""""""""
		err := collection.FindOne(ctx, bson.M{"clientid": ViewStruct.ClientId, "doctorid": ViewStruct.DoctorId}).Decode(&ViewStructDecode)
		if err != nil {
			log.Printf("Find ERR views%v\n", err)
		}
		collectionToDoc.FindOne(ctx, bson.M{"_id": ViewStruct.DoctorId}).Decode(&DoctorDecode)
		collectionToDoc.FindOne(ctx, bson.M{"_id": ViewStruct.ClientId}).Decode(&DoctorDecodeUser)
		fmt.Println(DoctorDecodeUser)
		if isPassedFields == true && ViewStructDecode.Sickness == "" {
			// ? prepeare for inserting the data
			premetivid := primitive.NewObjectID().Hex()
			ViewStruct.Id = premetivid
			ViewStruct.DoctorPhone = DoctorDecode.Phone
			ViewStruct.ClientFLSname = fmt.Sprintf("%v %v", DoctorDecodeUser.Surname, DoctorDecodeUser.Name)
			ViewStruct.DoctorFLSname = DoctorDecode.Surname + " " + DoctorDecode.Name
			ViewStruct.Date = dateZoneFormat
			_, err := collection.InsertOne(ctx, ViewStruct)
			if err != nil {
				log.Printf("Insert ERR views%v\n", err)
			}
			// ? Remove views from DB
			go removeViewsFromDB(ViewStruct.Id)
		} else {
			c.JSON(400, gin.H{
				"Code": "You Just pusted such request",
			})
		}
	}
}
func removeViewsFromDB(id string) {
	var (
		DecodedViews structures.Views
	)
	Authenticationservice()
	conn := client.Database("MedCard").Collection("views")
	conn.FindOne(ctx, bson.M{"_id": id}).Decode(&DecodedViews)

	//? Create time new zone  forat rf3399 2023-05-28T17:23:00+05:00
	offsetTime := time.FixedZone("Tajikistan", 5*3600)
	now := time.Now().In(offsetTime)
	//? Colc all time in second
	timeParse, err := time.Parse(time.RFC3339, DecodedViews.Date)
	if err != nil {
		fmt.Printf("Error parse the time%v", err)
	}
	// ? Time managment if it expired tomrrow or today
	var MinutesForRm int
	fmt.Println(((((timeParse.Hour() - now.Hour()) * 60) + (timeParse.Minute() - now.Minute())) + 1) + (timeParse.Day()-now.Day())*1440)

	if now.After(timeParse) == false {
		MinutesForRm = ((((timeParse.Hour() - now.Hour()) * 60) + (timeParse.Minute() - now.Minute())) + 1) + (timeParse.Day()-now.Day())*1440
		fmt.Printf("Access will be denied after %v minutes 1", MinutesForRm)
	}
	// ? Set deley after which delete remove access for view
	if DecodedViews.Date != "" {
		select {
		case <-time.After(time.Duration(MinutesForRm) * time.Minute):
			var (
				DecodedViewsForArchive structures.Views
			)
			// ? Get the data from DB
			conn.FindOne(ctx, bson.M{"_id": id}).Decode(&DecodedViewsForArchive)
			// *
			parseTimeFromDb, _ := time.Parse(time.RFC3339, DecodedViewsForArchive.Date)
			// ? validate access data
			offsetTime := time.FixedZone("Tajikistan", 5*3600)
			now := time.Now().In(offsetTime)
			if now.After(parseTimeFromDb) == true {
				connArch := client.Database("MedCard").Collection("viewsarchive")
				connArch.InsertOne(ctx, DecodedViewsForArchive)

				conn.DeleteOne(ctx, bson.M{"_id": id})
				fmt.Println("User  access has been removed removed")
			} else {
				fmt.Println("Access time has been changed")
			}

		}

	} else {
		fmt.Println("No date")
	}
}

func AddFilesToEhr(c *gin.Context) {
	var (
		FilesStruct  structures.File
		DecodeClient structures.Signup
		DecodeViews  structures.Views
	)
	CookieData := jwtgen.Velidation(c)
	stringJSON := c.Request.FormValue("json")
	files, _, errIMG := c.Request.FormFile("img")
	// """""""""""""""""""""""check The file on existense"""""""""""""""""""""""
	if errIMG != nil {
		c.JSON(409, gin.H{
			"sttus": "NOIMGFILEEXIST",
		})
	}

	files.Seek(23, 23)
	json.Unmarshal([]byte(stringJSON), &FilesStruct)
	jsStr, err := json.Marshal(FilesStruct)
	if err != nil {
		log.Printf("%v", err)
	}
	// """"""""""""""""""""""""""""""""""DB CONNECTION""""""""""""""""""""""""""""""""""""""""""""""""""""
	Authenticationservice()
	collection := client.Database("MedCard").Collection("ehrfiles")
	collectionUsers := client.Database("MedCard").Collection("users")
	collectionUsers.FindOne(ctx, bson.M{"_id": CookieData.Id}).Decode(&DecodeClient)
	collectionviews := client.Database("MedCard").Collection("views")
	// """"""""""""""""""""""""""""""""""DB CONNECTION""""""""""""""""""""""""""""""""""""""""""""""""""""
	isPassedFields, _ := velidation.TestTheStruct(c, "clientFLSname:clientid:doctorid:description:doctorFLSname:title", string(jsStr), "FieldsCheck:true,DBCheck:false", "", "")
	if isPassedFields == true {
		// """""""""""""""""""""""""Check Access if client do else check views for access"""""""""""""""""""""""""""""""""""""""
		if CookieData.Permissions == "client" && FilesStruct.ClientId == CookieData.Id || CookieData.Permissions == "admin" {
			premetivid := primitive.NewObjectID().Hex()
			FilesStruct.Id = premetivid
			FilesStruct.ImgUrl = handlefile.Handlefile(c, "./static/uploadfille")
			collection.InsertOne(ctx, FilesStruct)
			c.JSON(200, gin.H{
				"Code": "Request Seccessfully Handleed",
			})
		} else if CookieData.Permissions == "doctor" {
			collectionviews.FindOne(ctx, bson.M{"doctorid": CookieData.Id, "clientid": FilesStruct.ClientId}).Decode(&DecodeViews)
			if DecodeViews.Sickness != "" && DecodeViews.Date != "" {
				premetivid := primitive.NewObjectID().Hex()
				FilesStruct.Id = premetivid
				FilesStruct.ImgUrl = handlefile.Handlefile(c, "./static/uploadfille")
				collection.InsertOne(ctx, FilesStruct)
				c.JSON(200, gin.H{
					"Code": "Request Seccessfully Handleed",
				})
			} else {
				c.JSON(400, gin.H{
					"Code": "You havve no access to add file to that user",
				})
			}
		} else {
			c.JSON(400, gin.H{
				"Code": "You cannot add file to this user",
			})
		}
	}
}

// ! Make the file expired by colling after some time
func ExpiredLinks(c *gin.Context) {
	//! http://127.0.0.1:5500/link?client=6468f42e1b2b6c995ac8dfc8&id=345464489.jpg&type=client
	typeOfFile := c.Request.URL.Query().Get("type")
	if typeOfFile == "doctor" {
		staticFiles(c, "./static/doctors/upload-", "")
	} else if typeOfFile == "client" {
		cookieData := jwtgen.Velidation(c)
		if cookieData.Permissions == "client" || cookieData.Permissions == "doctor" {
			staticFiles(c, "./static/upload/upload-", cookieData.Id)
		}
	}
}
func staticFiles(c *gin.Context, path string, id string) {
	fmt.Println(time.Now())
	imgId := c.Request.URL.Query().Get("id")
	clientId := c.Request.URL.Query().Get("client")
	// parsing time "2023-5-22T18:20:00Z05:00" as "2006-01-02T15:04:05Z07:00": cannot parse "5-22T18:20:00Z05:00" as "01"
	var (
		ehrfiles  structures.File
		viewsList structures.Views
	)
	//?  If the id  does'nt exist that meanes that request wonts to get doctors image as they are publicly accessible give the withouth validation else it is client validate it before giving access
	if id != "" {
		Authenticationservice()
		conn := client.Database("MedCard").Collection("ehrfiles")
		conn.FindOne(ctx, bson.M{"clientid": id, "imgurl": "-" + imgId}).Decode(&ehrfiles)
		// ? Check if the client request the img else validate it first
		if ehrfiles.ImgUrl == "-"+imgId {
			http.ServeFile(c.Writer, c.Request, path+imgId)
		} else {
			connView := client.Database("MedCard").Collection("views")
			connView.FindOne(ctx, bson.M{"clientid": clientId, "doctorid": id}).Decode(&viewsList)

			if viewsList.ClientId == id || viewsList.DoctorId == id {

				NewTimeZone := time.FixedZone("Tajikistan", 5*3600)
				tajikistanTimeZone := time.Now().UTC().In(NewTimeZone)

				ParseTime, err := time.Parse(time.RFC3339, viewsList.Date)
				if err != nil {
					fmt.Println(err)
				}
				fmt.Printf("tajikistanTimeZone: %v\n", tajikistanTimeZone)
				fmt.Printf("expireTime: %v\n", ParseTime)
				if tajikistanTimeZone.After(ParseTime) {
					c.JSON(404, gin.H{
						"Code": "Your session to this file expired",
					})
				} else {
					http.ServeFile(c.Writer, c.Request, path+imgId)
				}
			} else {
				c.JSON(404, gin.H{
					"Code": "Your session to this file expired || You have no access to it",
				})
			}
		}

	} else {
		http.ServeFile(c.Writer, c.Request, path+imgId)
	}
}
