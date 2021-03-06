package consensus

/*

1. 去配置的数据节点取数据 rpc
2. 如果数据节点当前高度为 0，那么
3. 如果是Single节点，制作 1-10 高度的区块， 发送到数据节点，
4. 如果数据节点返回的高度不为 0， 那么需要同步数据节点的区块，
5. 同步后，开始共识，生产区块。

*/

// 同步过程
/*
1. 节点启动后，读取自己存储的lastBlockHeader, 如果没有读取成功，则认为是新节点，从区块 0 开始，否则
2. 从最后区块高度开始， 向数据节点请求节点本地最后高度之后的区块；
3. 同步完成后，开始共识。
4. 当在共识过程中，发现自己落后，那么向数据节点请求最新的区块。


*/