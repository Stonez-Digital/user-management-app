package controller

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/httpx"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/gorm"
)

type SubscriptionBillingController struct {
	billing *service.SubscriptionBillingService
	subscriptions *service.SubscriptionService
	db *gorm.DB
}

func NewSubscriptionBillingController(db *gorm.DB, provider service.SubscriptionPaymentProvider) *SubscriptionBillingController {
	return &SubscriptionBillingController{billing:service.NewSubscriptionBillingService(db,provider),subscriptions:service.NewSubscriptionService(db),db:db}
}

type subscriptionCheckoutRequest struct {
	PlanCode string `json:"plan_code" binding:"required,max=30"`
	TermID uuid.UUID `json:"term_id" binding:"required"`
}
type subscriptionVerifyRequest struct {
	Reference string `json:"reference" binding:"required,max=100"`
}

func (ctrl *SubscriptionBillingController) Plans(c *gin.Context) {
	plans,err:=ctrl.subscriptions.ListPlans()
	if err!=nil { httpx.Error(c,http.StatusInternalServerError,"billing_plans_unavailable","failed to load billing plans");return }
	c.JSON(http.StatusOK,plans)
}

func (ctrl *SubscriptionBillingController) Current(c *gin.Context) {
	schoolID,ok:=middleware.SchoolIDFromContext(c);if !ok {httpx.Error(c,http.StatusForbidden,"school_context_required","school context required");return}
	sub,err:=ctrl.subscriptions.CurrentSubscription(schoolID)
	if errors.Is(err,service.ErrSubscriptionNotFound) {httpx.Error(c,http.StatusNotFound,"subscription_not_found","no active subscription");return}
	if err!=nil {httpx.Error(c,http.StatusInternalServerError,"subscription_unavailable","failed to load subscription");return}
	c.JSON(http.StatusOK,sub)
}

func (ctrl *SubscriptionBillingController) Checkout(c *gin.Context) {
	schoolID,ok:=middleware.SchoolIDFromContext(c);if !ok {httpx.Error(c,http.StatusForbidden,"school_context_required","school context required");return}
	var req subscriptionCheckoutRequest
	if err:=c.ShouldBindJSON(&req);err!=nil {httpx.Validation(c,httpx.ValidationErrors(err));return}
	actorID,err:=uuid.Parse(c.GetString(middleware.UserIDKey))
	if err!=nil {httpx.Error(c,http.StatusUnauthorized,"invalid_user_context","authentication required");return}
	var actor models.User
	if err:=ctrl.db.Select("id","email","school_id","active").Where("id = ? AND school_id = ?",actorID,schoolID).First(&actor).Error;err!=nil || !actor.Active || strings.TrimSpace(actor.Email)=="" {
		httpx.Error(c,http.StatusForbidden,"billing_contact_unavailable","an active school administrator email is required");return
	}
	result,err:=ctrl.billing.CreateCheckout(schoolID,req.TermID,req.PlanCode,actor.Email,os.Getenv("PAYSTACK_CALLBACK_URL"))
	if err!=nil {
		switch {
		case errors.Is(err,service.ErrSubscriptionTermUnavailable): httpx.Error(c,http.StatusConflict,"term_unavailable","select a current active term belonging to this school")
		case errors.Is(err,service.ErrSubscriptionPlanUnavailable): httpx.Error(c,http.StatusBadRequest,"plan_unavailable","selected plan is unavailable")
		case errors.Is(err,service.ErrSubscriptionPaymentPending): httpx.Error(c,http.StatusConflict,"checkout_pending","a subscription checkout is already pending")
		case errors.Is(err,service.ErrSubscriptionPaymentRecoveryRequired): httpx.Error(c,http.StatusServiceUnavailable,"checkout_recovery_required","the existing checkout needs reconciliation; if payment was completed, verify using its original reference")
		case errors.Is(err,service.ErrSubscriptionPaymentAlreadyActive): httpx.Error(c,http.StatusConflict,"subscription_active","the school already has an active subscription")
		case errors.Is(err,service.ErrSubscriptionPaymentUnavailable): httpx.Error(c,http.StatusServiceUnavailable,"checkout_unavailable","subscription checkout is not available")
		default: httpx.Error(c,http.StatusInternalServerError,"checkout_failed","could not initialize subscription checkout")
		}
		return
	}
	c.JSON(http.StatusCreated,gin.H{"payment":result.Payment,"subscription":result.Subscription,"authorization_url":result.Payment.AuthorizationURL,"reference":result.Payment.Reference})
}

func (ctrl *SubscriptionBillingController) Verify(c *gin.Context) {
	schoolID,ok:=middleware.SchoolIDFromContext(c);if !ok {httpx.Error(c,http.StatusForbidden,"school_context_required","school context required");return}
	var req subscriptionVerifyRequest
	if err:=c.ShouldBindJSON(&req);err!=nil {httpx.Validation(c,httpx.ValidationErrors(err));return}
	result,err:=ctrl.billing.VerifyCheckout(schoolID,req.Reference)
	if err!=nil {
		switch {
		case errors.Is(err,service.ErrSubscriptionPaymentNotFound): httpx.Error(c,http.StatusNotFound,"payment_not_found","subscription payment reference not found")
		case errors.Is(err,service.ErrSubscriptionPaymentAlreadyActive): httpx.Error(c,http.StatusConflict,"subscription_active","another subscription is already active")
		case errors.Is(err,service.ErrSubscriptionPaymentInvalid): httpx.Error(c,http.StatusConflict,"payment_not_verified","payment could not be verified")
		default: httpx.Error(c,http.StatusInternalServerError,"payment_verification_failed","failed to verify subscription payment")
		}
		return
	}
	c.JSON(http.StatusOK,gin.H{"payment":result.Payment,"subscription":result.Subscription,"status":"active"})
}
