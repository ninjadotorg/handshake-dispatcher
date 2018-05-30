package server

import (
    "fmt"
    "net/http"
    "net/http/httputil"
    "net/url"
    "time"
    "log"
    "strconv"
    "github.com/gin-gonic/gin"

    "github.com/autonomousdotai/handshake-dispatcher/controllers"
    "github.com/autonomousdotai/handshake-dispatcher/middlewares"
    "github.com/autonomousdotai/handshake-dispatcher/config"
    "github.com/autonomousdotai/handshake-dispatcher/models"
)

func NewRouter() *gin.Engine {
    router := gin.New()
    router.Use(gin.Logger())
    router.Use(middlewares.CORSMiddleware())
    router.Use(middlewares.ErrorHandler())
    router.Use(middlewares.ChainMiddleware())

    defaultController := new(controllers.DefaultController)
    router.GET("/", defaultController.Home) 

    userController := new(controllers.UserController)
    userGroup := router.Group("user")
    {
        userGroup.GET("/profile", middlewares.AuthMiddleware(), userController.Profile)

        userGroup.POST("/profile", middlewares.AuthMiddleware(), userController.UpdateProfile)

        userGroup.POST("/sign-up", userController.SignUp)
    }

    handshakeController := new(controllers.HandshakeController)
    handshakeGroup := router.Group("handshake")
    {
        handshakeGroup.GET("/me", middlewares.AuthMiddleware(), handshakeController.Me)
        handshakeGroup.GET("/discover", handshakeController.Discover)
        handshakeGroup.POST("/create", handshakeController.Create)
        handshakeGroup.POST("/update", handshakeController.Update)
        handshakeGroup.POST("/delete", handshakeController.Delete)
    }


    systemController := new(controllers.SystemController)
    systemGroup := router.Group("system")
    {
        systemGroup.GET("/user/:id", systemController.User)
    }

    conf := config.GetConfig()
    for ex, ep := range conf.GetStringMap("forwarding") { 
        endpoint := ep
        router.Any(ex, middlewares.AuthMiddleware(), func(c *gin.Context) {
            Forwarding(c, &endpoint, "")
        })
        router.Any(ex + "/*path", middlewares.AuthMiddleware(), func(c *gin.Context) {
            path := c.Param("path")
            Forwarding(c, &endpoint, path)
        })
    }

    router.NoRoute(defaultController.NotFound)

    return router
}

type ForwardingTransport struct {
}

func (t *ForwardingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
    start := time.Now()
    response, err := http.DefaultTransport.RoundTrip(request)

    if err != nil {
        fmt.Println("\n\ncame in error resp here", err)
        return nil, err
    }

    elapsed := time.Since(start)
    
    _, err = httputil.DumpResponse(response, true)
    if err != nil {
        fmt.Println("\n\ndump response error");
    }

    log.Printf("%s - %d\n", request.Method + request.URL.Path, elapsed.Nanoseconds)
    return response, nil
}

func Forwarding(c *gin.Context, endpoint *interface{}, path string) { 
    r := c.Request
    w := c.Writer
    
    user, _ := c.Get("User")

    url, _ := url.Parse((*endpoint).(string) + path)
    director := func(req *http.Request) {
        req.URL.Scheme = url.Scheme
        req.URL.Host = url.Host
        req.URL.Path = url.Path

        for k, _ := range r.Header {
            v := c.GetHeader(k)
            req.Header.Set(k, v)
        }
        req.Header.Set("uid", strconv.FormatUint(uint64((user.(models.User)).ID), 10))
    }
    proxy := &httputil.ReverseProxy{Director: director} 
    proxy.Transport = &ForwardingTransport{}
    proxy.ServeHTTP(w, r)
}
