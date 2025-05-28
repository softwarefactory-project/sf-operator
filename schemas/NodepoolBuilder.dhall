{ Type =
    { limits : Optional (./Limits.dhall).Type
    , logLevel : Optional Text
    , storage : Optional (./Storage.dhall).Type
    }
, default =
  { limits = None (./Limits.dhall).Type
  , logLevel = None Text
  , storage = None (./Storage.dhall).Type
  }
}