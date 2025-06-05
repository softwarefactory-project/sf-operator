{ Type =
    { limits : Optional (./Limits.dhall).Type
    , storage : Optional (./Storage.dhall).Type
    }
, default =
  { limits = None (./Limits.dhall).Type, storage = None (./Storage.dhall).Type }
}