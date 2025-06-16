{ Type =
    { name : Text
    , uri : Text
    , basicAuthSecret : Optional Text
    , useSSL : Optional Bool
    , verifyCerts : Optional Bool
    }
, default =
  { basicAuthSecret = None Text, useSSL = None Bool, verifyCerts = None Bool }
}