{ Type =
    { DefaultHoldExpiration : Optional Natural
    , MaxHoldExpiration : Optional Natural
    , limits : Optional (./Limits.dhall).Type
    , logLevel : Optional Text
    , statsdTarget : Optional Text
    , storage : Optional (./Storage.dhall).Type
    }
, default =
  { DefaultHoldExpiration = None Natural
  , MaxHoldExpiration = None Natural
  , limits = None (./Limits.dhall).Type
  , logLevel = None Text
  , statsdTarget = None Text
  , storage = None (./Storage.dhall).Type
  }
}