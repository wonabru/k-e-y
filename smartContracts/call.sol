%DEPLOYED_CONTRACT%

contract Proxy {
    address deployed_contract = %CONTRACT_ADDRESS%;

    function directCall() public {
        %CONTRACT_NAME%(deployed_contract).%CONTRACT_FUNC%%INPUT_DATA%;
    }
}
