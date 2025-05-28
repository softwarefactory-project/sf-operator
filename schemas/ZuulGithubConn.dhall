{ Type =
    { name : Text
    , appID : Optional Natural
    , canonicalHostname : Optional Text
    , secrets : Optional Text
    , server : Optional Text
    , verifySsl : Optional Bool
    }
, default =
  { appID = None Natural
  , canonicalHostname = None Text
  , secrets = None Text
  , server = None Text
  , verifySsl = None Bool
  }
}