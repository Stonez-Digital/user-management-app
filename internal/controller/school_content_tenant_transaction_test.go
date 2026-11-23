package controller

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/middleware"
)

func TestSchoolContentAdminHandlersFailClosedWithoutTenantTransaction(t *testing.T) {
    gin.SetMode(gin.TestMode)
    ctrl := NewSchoolContentController(nil, nil, nil)
    cases := []struct { name, method, path string; handler gin.HandlerFunc }{
        {"list advertisements", http.MethodGet, "/advertisements", ctrl.ListAdvertisements},
        {"create advertisement", http.MethodPost, "/advertisements", ctrl.CreateAdvertisement},
        {"update advertisement", http.MethodPut, "/advertisements/:id", ctrl.UpdateAdvertisement},
        {"delete advertisement", http.MethodDelete, "/advertisements/:id", ctrl.DeleteAdvertisement},
        {"upload advertisement image", http.MethodPost, "/advertisements/:id/image", ctrl.UploadAdvertisementImage},
        {"list blogs", http.MethodGet, "/blog/posts", ctrl.ListBlogs},
        {"create blog", http.MethodPost, "/blog/posts", ctrl.CreateBlog},
        {"update blog", http.MethodPut, "/blog/posts/:id", ctrl.UpdateBlog},
        {"delete blog", http.MethodDelete, "/blog/posts/:id", ctrl.DeleteBlog},
        {"publish blog", http.MethodPost, "/blog/posts/:id/publish", ctrl.PublishBlog},
        {"upload blog image", http.MethodPost, "/blog/posts/:id/image", ctrl.UploadBlogImage},
        {"list albums", http.MethodGet, "/gallery/albums", ctrl.ListAlbums},
        {"create album", http.MethodPost, "/gallery/albums", ctrl.CreateAlbum},
        {"update album", http.MethodPut, "/gallery/albums/:id", ctrl.UpdateAlbum},
        {"delete album", http.MethodDelete, "/gallery/albums/:id", ctrl.DeleteAlbum},
        {"upload gallery image", http.MethodPost, "/gallery/albums/:id/images", ctrl.UploadImage},
        {"delete gallery image", http.MethodDelete, "/gallery/images/:id", ctrl.DeleteImage},
        {"list enquiries", http.MethodGet, "/enquiries", ctrl.ListEnquiries},
        {"update enquiry", http.MethodPatch, "/enquiries/:id", ctrl.UpdateEnquiryStatus},
        {"delete enquiry", http.MethodDelete, "/enquiries/:id", ctrl.DeleteEnquiry},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            r := gin.New()
            r.Use(func(c *gin.Context) { c.Set(middleware.SchoolIDKey, uuid.New().String()); c.Next() })
            r.Handle(tc.method, tc.path, tc.handler)
            req := httptest.NewRequest(tc.method, "/"+strings.TrimPrefix(tc.path, "/"), nil)
            if strings.Contains(tc.path, ":id") { req = httptest.NewRequest(tc.method, strings.Replace("/"+strings.TrimPrefix(tc.path, "/"), ":id", uuid.NewString(), 1), nil) }
            rec := httptest.NewRecorder()
            r.ServeHTTP(rec, req)
            if rec.Code != http.StatusInternalServerError { t.Fatalf("expected fail-closed 500, got %d: %s", rec.Code, rec.Body.String()) }
            if !strings.Contains(rec.Body.String(), "tenant_transaction_required") { t.Fatalf("expected tenant_transaction_required, got %s", rec.Body.String()) }
        })
    }
}
