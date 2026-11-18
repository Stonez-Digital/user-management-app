package controller
import ("net/http"; "github.com/gin-gonic/gin"; "github.com/google/uuid"; "github.com/onoja217/users-management-app/internal/models"; "github.com/onoja217/users-management-app/internal/service")
type NationalCurriculumController struct{svc *service.NationalCurriculumService}
func NewNationalCurriculumController(svc *service.NationalCurriculumService)*NationalCurriculumController{return &NationalCurriculumController{svc}}
func(c *NationalCurriculumController)List(ctx *gin.Context){v,e:=c.svc.List();if e!=nil{ctx.JSON(http.StatusInternalServerError,gin.H{"error":"failed to list national curricula"});return};ctx.JSON(http.StatusOK,v)}
func(c *NationalCurriculumController)Get(ctx *gin.Context){v,e:=c.svc.Get(ctx.Param("id"));if e!=nil{ctx.JSON(http.StatusBadRequest,gin.H{"error":"invalid national curriculum id"});return};ctx.JSON(http.StatusOK,v)}
func(c *NationalCurriculumController)Create(ctx *gin.Context){var v models.NationalCurriculum;if e:=ctx.ShouldBindJSON(&v);e!=nil{ctx.JSON(http.StatusBadRequest,gin.H{"error":"invalid request"});return};v.ID=uuid.Nil; out,e:=c.svc.Create(v);if e!=nil{ctx.JSON(http.StatusBadRequest,gin.H{"error":e.Error()});return};ctx.JSON(http.StatusCreated,out)}
