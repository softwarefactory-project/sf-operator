{ Type =
    { debug : Optional Bool
    , forwardInputHost : Optional Text
    , forwardInputPort : Optional Natural
    }
, default =
  { debug = None Bool
  , forwardInputHost = None Text
  , forwardInputPort = None Natural
  }
}