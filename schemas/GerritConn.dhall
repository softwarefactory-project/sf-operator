{ Type =
    { hostname : Text
    , name : Text
    , canonicalhostname : Optional Text
    , git-over-ssh : Optional Bool
    , password : Optional Text
    , port : Optional Natural
    , puburl : Optional Text
    , sshkey : Optional Text
    , stream-events : Optional Bool
    , username : Optional Text
    , verifyssl : Optional Bool
    }
, default =
  { canonicalhostname = None Text
  , git-over-ssh = None Bool
  , password = None Text
  , port = None Natural
  , puburl = None Text
  , sshkey = None Text
  , stream-events = None Bool
  , username = None Text
  , verifyssl = None Bool
  }
}