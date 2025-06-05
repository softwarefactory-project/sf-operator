{ Type =
    { dbStorage : Optional (./Storage.dhall).Type
    , limits : Optional (./Limits.dhall).Type
    , logStorage : Optional (./Storage.dhall).Type
    }
, default =
  { dbStorage = None (./Storage.dhall).Type
  , limits = None (./Limits.dhall).Type
  , logStorage = None (./Storage.dhall).Type
  }
}