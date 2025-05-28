{ Type =
    { clientID : Text
    , issuerID : Text
    , name : Text
    , realm : Text
    , audience : Optional Text
    , authority : Optional Text
    , keysURL : Optional Text
    , loadUserInfo : Optional Bool
    , maxValidityTime : Optional Natural
    , scope : Optional Text
    , skew : Optional Natural
    , uidClaim : Optional Text
    }
, default =
  { audience = None Text
  , authority = None Text
  , keysURL = None Text
  , loadUserInfo = None Bool
  , maxValidityTime = None Natural
  , scope = None Text
  , skew = None Natural
  , uidClaim = None Text
  }
}