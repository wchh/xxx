package chain

/**

1. chain用来存储block和tx数据
2. block 只存储 block header 和 txs 的 hash list, storeBlock
3. chain 维护一个 preBlock 列表, 用来存储用户没有共识的tx
3. 用户的 tx 存储在某个 高度的 preBlock 里, 具体是当前高度+N(N 取值为 4，理想情况 N 应该为 1)
4. 共识节点通过rpc 获取下一个 preBlock
5. 共识节点通过rpc 把共识的区块发送给 chain
6. tx 查重，因为tx依靠hash存储，所以需要确保tx的唯一性。对于新的 tx 中，包含height字段，height必须大于当前共识高度.

**/
