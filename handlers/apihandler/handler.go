package apihandler

import (
	"medcard-new/begening/controllers/authenticationservice"
	"medcard-new/begening/controllers/ehrcontroller"
	controllers "medcard-new/begening/controllers/handlersservice"

	"github.com/gin-gonic/gin"
)

func Handlers(){
	r := gin.Default()

	// r.StaticFS("/static", gin.Dir("./static", true))
	r.StaticFS("/static", gin.Dir("./static", false))
	r.Use(controllers.Cors)

	r.POST("/insertquestion",controllers.InsertQuestions)
	r.POST("/profilechange",controllers.ProfileChange)
	r.GET("/link",ehrcontroller.ExpiredLinks)
	// ================================== New Route ==============================

	r.POST("/signup",authenticationservice.Signup)
	r.POST("/reset",authenticationservice.ResetPassword)
	r.POST("/signin",authenticationservice.Signin)
	r.POST("/signout",authenticationservice.Signout)
	r.POST("/logincheck",authenticationservice.LoginCheck)
	r.POST("/signupdoctor",authenticationservice.SignupDoctor)
	r.POST("/handleviews",ehrcontroller.DoctorClientForView)
	r.POST("/filesadd",ehrcontroller.AddFilesToEhr)
	
	r.GET("/getclient",controllers.GetClient)
	r.GET("/getquestion",controllers.GetQuestions)
	r.GET("/getdoctors",controllers.GetDoctors)
	r.GET("/statistics",controllers.Statistics)
	r.GET("/getclients",controllers.GetClients)
	r.GET("/getviews",controllers.GetViews)
	r.GET("/listviews",controllers.ListViewsAdmin)

	
	r.Run(":5500")
}
