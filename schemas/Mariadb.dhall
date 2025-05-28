{ Type =
    { dbStorage : (./Storage.dhall).Type
    , logStorage : (./Storage.dhall).Type
    , limits : Optional (./Limits.dhall).Type
    }
, default.limits = None (./Limits.dhall).Type
}