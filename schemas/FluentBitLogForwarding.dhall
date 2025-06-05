{ Type =
    { forwardInputHost : Text
    , debug : Optional Bool
    , forwardInputPort : Optional Natural
    }
, default = { debug = None Bool, forwardInputPort = None Natural }
}