{- The operator configuration.

Note: run `make config-update` to update files before commiting changes
 -}
let version = "0.0.2"

in  { year = "2020"
    , author = "Red Hat"
    , version = version
    , image = "quay.io/software-factory/sf-operator:${version}"
    , group = "softwarefactory-project.io"
    , crd =
        { kind = "SoftwareFactory"
        , plural = "softwarefactories"
        , singular = "softwarefactory"
        , role = "sf"
        }
    }
