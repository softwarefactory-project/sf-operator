{ Type =
    { diskLimitPerJob : Optional Integer
    , enabled : Optional Bool
    , limits : Optional (./Limits.dhall).Type
    , logLevel : Optional Text
    , standalone :
        Optional
          { controlPlanePublicGSHostname : Text
          , controlPlanePublicZKHostname : Text
          , publicHostname : Text
          }
    , storage : Optional (./Storage.dhall).Type
    }
, default =
  { diskLimitPerJob = None Integer
  , enabled = None Bool
  , limits = None (./Limits.dhall).Type
  , logLevel = None Text
  , standalone =
      None
        { controlPlanePublicGSHostname : Text
        , controlPlanePublicZKHostname : Text
        , publicHostname : Text
        }
  , storage = None (./Storage.dhall).Type
  }
}