{ Type =
    { gitHttpLowSpeedLimit : Optional Natural
    , gitHttpLowSpeedTime : Optional Natural
    , gitTimeout : Optional Natural
    , gitUserEmail : Optional Text
    , gitUserName : Optional Text
    , limits : Optional (./Limits.dhall).Type
    , logLevel : Optional Text
    , storage : Optional (./Storage.dhall).Type
    }
, default =
  { gitHttpLowSpeedLimit = None Natural
  , gitHttpLowSpeedTime = None Natural
  , gitTimeout = None Natural
  , gitUserEmail = None Text
  , gitUserName = None Text
  , limits = None (./Limits.dhall).Type
  , logLevel = None Text
  , storage = None (./Storage.dhall).Type
  }
}