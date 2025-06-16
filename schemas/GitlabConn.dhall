{ Type =
    { name : Text
    , secrets : Text
    , apiTokenName : Optional Text
    , baseUrl : Optional Text
    , canonicalHostname : Optional Text
    , cloneUrl : Optional Text
    , disableConnectionPool : Optional Bool
    , keepAlive : Optional Natural
    , server : Optional Text
    }
, default =
  { apiTokenName = None Text
  , baseUrl = None Text
  , canonicalHostname = None Text
  , cloneUrl = None Text
  , disableConnectionPool = None Bool
  , keepAlive = None Natural
  , server = None Text
  }
}