{ Type =
    { name : Text
    , appName : Optional Text
    , baseUrl : Optional Text
    , canonicalHostname : Optional Text
    , cloneUrl : Optional Text
    , secrets : Optional Text
    , server : Optional Text
    , sourceWhitelist : Optional Text
    }
, default =
  { appName = None Text
  , baseUrl = None Text
  , canonicalHostname = None Text
  , cloneUrl = None Text
  , secrets = None Text
  , server = None Text
  , sourceWhitelist = None Text
  }
}