{ Type =
    { name : Text
    , server : Text
    , defaultFrom : Optional Text
    , defaultTo : Optional Text
    , password : Optional Text
    , port : Optional Natural
    , secrets : Optional Text
    , tls : Optional Bool
    , user : Optional Text
    }
, default =
  { defaultFrom = None Text
  , defaultTo = None Text
  , password = None Text
  , port = None Natural
  , secrets = None Text
  , tls = None Bool
  , user = None Text
  }
}