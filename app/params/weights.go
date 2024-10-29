package params

const (
    WeightMsgSend                        = 100
    WeightMsgMultiSend                   = 10
    WeightMsgDeposit                     = 100
    WeightMsgVote                        = 67

    WeightMsgDelegate                    = 100
    WeightMsgUndelegate                  = 100
    WeightMsgBeginRedelegate            = 100
    WeightMsgCreateValidator            = 100
    WeightMsgEditValidator              = 5
    WeightMsgUnjail                     = 100

    WeightMsgSetWithdrawAddress         = 50
    WeightMsgWithdrawDelegationReward   = 50
    WeightMsgWithdrawValidatorCommission = 50
    WeightMsgFundCommunityPool          = 50

    WeightMsgStoreCode                  = 50
    WeightMsgInstantiateContract        = 100
    WeightMsgExecuteContract            = 100
    WeightMsgUpdateAdmin                = 25
    WeightMsgClearAdmin                 = 10
    WeightMsgMigrateContract            = 50

    WeightProposalCommunitySpend        = 5
    WeightProposalText                  = 5
    WeightProposalParamChange           = 5
    WeightProposalStoreCode             = 5
    WeightProposalInstantiateContract   = 5
    WeightProposalUpdateAdmin           = 5
    WeightProposalExecuteContract       = 5
    WeightProposalClearAdmin            = 5
    WeightProposalMigrateContract       = 5
    WeightProposalSudoContract          = 5
    WeightProposalPinCodes              = 5
    WeightProposalUnpinCodes            = 5
    WeightProposalInstantiateConfig     = 5
    WeightProposalStoreAndInstantiate   = 5
)
