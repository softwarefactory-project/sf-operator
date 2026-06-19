{ Type =
    { TerminationGracePeriodSeconds : Optional Natural
    , diskLimitPerJob : Optional Integer
    , ansibleSetupTimeout : Optional Integer
    , enabled : Optional Bool
    , limits : Optional (./Limits.dhall).Type
    , logLevel : Optional Text
    , standalone :
        Optional
          { controlPlanePublicGSHostname : Text
          , controlPlanePublicZKHostname : Text
          , publicHostname : Text
          , controlPlanePublicZKHostnames : Optional (List Text)
          , zone : Optional Text
          }
    , storage : Optional (./Storage.dhall).Type
    }
, default =
  { TerminationGracePeriodSeconds = None Natural
  , diskLimitPerJob = None Integer
  , ansibleSetupTimeout = None Integer
  , enabled = None Bool
  , limits = None (./Limits.dhall).Type
  , logLevel = None Text
  , standalone =
      None
        { controlPlanePublicGSHostname : Text
        , controlPlanePublicZKHostname : Text
        , publicHostname : Text
        , controlPlanePublicZKHostnames : Optional (List Text)
        , zone : Optional Text
        }
  , storage = None (./Storage.dhall).Type
  }
}