{ Type =
    { enabled : Optional Bool
    , limits : Optional (./Limits.dhall).Type
    , storage : Optional (./Storage.dhall).Type
    }
, default =
  { enabled = None Bool
  , limits = None (./Limits.dhall).Type
  , storage = None (./Storage.dhall).Type
  }
}