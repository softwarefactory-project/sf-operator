{ Type =
    { storage : (./Storage.dhall).Type
    , enabled : Optional Bool
    , limits : Optional (./Limits.dhall).Type
    }
, default = { enabled = None Bool, limits = None (./Limits.dhall).Type }
}