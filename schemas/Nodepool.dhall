{ Type =
    { builder : Optional (./NodepoolBuilder.dhall).Type
    , launcher : Optional (./NodepoolLauncher.dhall).Type
    , statsdTarget : Optional Text
    }
, default =
  { builder = None (./NodepoolBuilder.dhall).Type
  , launcher = None (./NodepoolLauncher.dhall).Type
  , statsdTarget = None Text
  }
}