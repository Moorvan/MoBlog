---
tags:
- 数据结构
- Go

title: "跳表"
date: 2022-07-30T13:32:44+08:00
draft: false
---


本文参考实现代码：[go-skiplist](https://github.com/gansidui/skiplist).

其中图片均引用自文章：[简书 skiplist](https://www.jianshu.com/p/9d8296562806).

# Intro

跳表是一种实用性很强的数据结构，其插入、删除、查找数据的时间复杂度和红黑树是同一量级，都是 $O(log(n))$，同时其维护的是一个有序的集合。在流行的缓存数据库 Redis 中选择使用跳表来作为其数据结构，支持以下的操作：

1. 插入一个元素
2. 删除一个元素
3. 查找一个元素
4. 有序输出所有元素
5. 按照给定区间查找元素（比如查找 key 值在 [100, 365] 之间的数据）

其中区间查找元素的需求在效率上，跳表表现要比红黑树更好。

# 设计

跳表数据结构可以看作是链表的扩展，对于链表结构为：

![Untitled](%E8%B7%B3%E8%A1%A8%200daa641ce789462d9196883cd1c83f3f/Untitled.png)

跳表底层也维护了一个有序的链表，通过给其建立一定的索引来加快链表的搜索效率，比如建立一层索引后其结构如图：

![Untitled](%E8%B7%B3%E8%A1%A8%200daa641ce789462d9196883cd1c83f3f/Untitled%201.png)

通过建立多层级的索引，来降低搜索效率，同时只有原始链表中存储了数据，索引中只存储 key 值和指针，因此当数据很大是，索引的开销会被忽略不计。

多层索引的搜索过程示意图：

![Untitled](%E8%B7%B3%E8%A1%A8%200daa641ce789462d9196883cd1c83f3f/Untitled%202.png)

可以看到相比链表，其查找效率被大大提高，但如果不去维护索引，而只是依赖一开始建立的索引进行增删改查，经过一段时间后，索引的分布可能会变得集中，最终使其退化为链表。

因此在跳表中，需要在插入数据同时维护索引，主要通过随机概率的方式，来使得各级索引不会退化。

如图：

![Untitled](%E8%B7%B3%E8%A1%A8%200daa641ce789462d9196883cd1c83f3f/Untitled%203.png)

对于插入数据 6，这里随机得到需要建立第一层和第二层索引，那么就会在利用索引树查找数据的过程中建立索引；其中这里的随机对于每层的随机概率不同，对于对于越上层的索引，应该概率越小。

# 代码实现

在我们的参考代码中，该数据结构分为两部分，首先是对基本数据结构的定义 Element，然后是对 skiplist 行为的定义。

## element

```go
type Element struct {
	Value    Interface
	backward *Element
	level    []*skiplistLevel
}

type skiplistLevel struct {
	forward *Element
	span    int
}
```

其通过 backward 存储其后节点，level[0].forward 中存储前节点，因此可以看到，这里维护了基础链表。

对于该链表，内部数据是私有的，提供对于元素前后访问的两个函数：

```go
// Next returns the next skiplist element or nil.
func (e *Element) Next() *Element {
	return e.level[0].forward
}

// Prev returns the previous skiplist element of nil.
func (e *Element) Prev() *Element {
	return e.backward
}
```

包内提供构造函数：

```go
// newElement returns an initialized element.
func newElement(level int, v Interface) *Element {
	slLevels := make([]*skiplistLevel, level)
	for i := 0; i < level; i++ {
		slLevels[i] = new(skiplistLevel)
	}

	return &Element{
		Value:    v,
		backward: nil,
		level:    slLevels,
	}
}
```

注意到传入构造函数两个参数，一个为 level，表示该 Element 对应的 level，在实例化是，构造一个 level 长度的 skiplistLevel 数组。对于其后节点的个数是很多个的，数量对应当前 Element 具有的索引层级。构造时候，对于每一层，即 level 数组的每一元素都存一个 skiplistLevel。注意到 level 表达的是总层数，包含了最底层（因此 level 最小为 1）。

包内提供得到随机层数的函数：

```go
const SKIPLIST_MAXLEVEL = 32
const SKIPLIST_BRANCH = 4

// randomLevel returns a random level.
func randomLevel() int {
	level := 1
	for (rand.Int31()&0xFFFF)%SKIPLIST_BRANCH == 0 {
		level += 1
	}

	if level < SKIPLIST_MAXLEVEL {
		return level
	} else {
		return SKIPLIST_MAXLEVEL
	}
}
```

初始 level 为 1，通过随机函数得到一个随机整数，取模 4 得 0 的概率为 1/4. 即 1/4 概率 +1，得，例如，有一层索引，即 $level \ge 2$ 的概率为：

$$
\frac{1}{4} + \frac{1}{4} \times \frac{1}{4} + \frac{1}{4} \times \frac{1}{4} \times \frac{1}{4} + ... = \frac{1}{2}
$$

## skiplist

在 element 中实际上存储的数据类型需要满足 skiplist.Interface 定义，即元素需要有一个序关系，因为 skiplist 底层维护的是一个有序链表，其接口定义为：

```go
type Interface interface {
	Less(other interface{}) bool
}
```

### 结构定义和初始化

接下来来看 skiplist 数据结构的实例化和初始化：

```go
type SkipList struct {
	header *Element
	tail   *Element
	update []*Element
	rank   []int
	length int
	level  int
}

// New returns an initialized skiplist.
func New() *SkipList {
	return &SkipList{
		header: newElement(SKIPLIST_MAXLEVEL, nil),
		tail:   nil,
		update: make([]*Element, SKIPLIST_MAXLEVEL),
		rank:   make([]int, SKIPLIST_MAXLEVEL),
		length: 0,
		level:  1,
	}
}

// Init initializes or clears skiplist sl.
func (sl *SkipList) Init() *SkipList {
	sl.header = newElement(SKIPLIST_MAXLEVEL, nil)
	sl.tail = nil
	sl.update = make([]*Element, SKIPLIST_MAXLEVEL)
	sl.rank = make([]int, SKIPLIST_MAXLEVEL)
	sl.length = 0
	sl.level = 1
	return sl
}
```

可以看到 Init 函数即将 skiplist 实例的状态恢复到 New 时的状态。其主要部分组成为 header 为一个 *`SKIPLIST_MAXLEVEL`* level 值的 Element，tail 为 nil，update 存放了一个 *`SKIPLIST_MAXLEVEL`* 长度的 *Element 数组，rank 存放了一个 *`SKIPLIST_MAXLEVEL`*长度的 int 数组，length 存 0， level 存 1.

接下来看一下对 skiplist 数据的基本查询方法：

```go
// Front returns the first elements of skiplist sl or nil.
func (sl *SkipList) Front() *Element {
	return sl.header.level[0].forward
}

// Back returns the last elements of skiplist sl or nil.
func (sl *SkipList) Back() *Element {
	return sl.tail
}

// Len returns the numbler of elements of skiplist sl.
func (sl *SkipList) Len() int {
	return sl.length
}
```

对于 skiplist ，提供了 Front 和 Back 函数，前者访问 header Element 的 level[0] 的 forward节点，Back 访问 tail 指针。存放了底层链表的首位。返回类型均为 *Element.

提供 Len() 方法访问数据结构存储数据的长度。

### 插入数据

接下来是整个数据结构维护的核心，即 Insert 方法，该方法同时需要向底层链表插入数据，同时通过随机过程来进行维护索引：

先看函数签名：

```go
func (sl *SkipList) Insert(v Interface) *Element
```

挂在 SkipList 类上的函数，会修改内部数据，传递实现 Interface 数据 v，返回一个 *Element。

插入首先更新 rank 数组和 update 数组：

```go
x := sl.header
for i := sl.level - 1; i >= 0; i-- {
	// store rank that is crossed to reach the insert position
	if i == sl.level-1 {
		sl.rank[i] = 0
	} else {
		sl.rank[i] = sl.rank[i+1]
	}
	for x.level[i].forward != nil && x.level[i].forward.Value.Less(v) {
		sl.rank[i] += x.level[i].span
		x = x.level[i].forward
	}
	sl.update[i] = x
}
```

这里将 rank 数组维护，最高层为 0，依次往下，用 x 进行搜索插入数据 v 的位置，并将过程中将 level[i] 中的 span 值附加到 rank[i] 中，并且每层的 rank[i] 初始值为高一层的 rank 值。对于每一层，存储搜索到最后位置的地方，并将 x 记录到 update[i] 中。最后位置的前节点值大于 v，即 update 存储最后一个小于 v 的节点。

接下来：

```go
// ensure that the v is unique, the re-insertion of v should never happen since the
// caller of sl.Insert() should test in the hash table if the element is already inside or not.
level := randomLevel()
if level > sl.level {
	for i := sl.level; i < level; i++ {
		sl.rank[i] = 0
		sl.update[i] = sl.header
		sl.update[i].level[i].span = sl.length
	}
	sl.level = level
}

x = newElement(level, v)
for i := 0; i < level; i++ {
	x.level[i].forward = sl.update[i].level[i].forward
	sl.update[i].level[i].forward = x

	// update span covered by update[i] as x is inserted here
	x.level[i].span = sl.update[i].level[i].span - sl.rank[0] + sl.rank[i]
	sl.update[i].level[i].span = sl.rank[0] - sl.rank[i] + 1
}

// increment span for untouched levels
for i := level; i < sl.level; i++ {
	sl.update[i].level[i].span++
}
```

通过随机函数得到该插入数据的 level，如果该 level 比当前实例的层数多，那么需要构建新层的索引，对于新建 i 层索引，那么需要将实例中 rank[i] 初始化为 0，update[i] 改为 header 指针，意味着这层的第一个元素即该待插入元素，更新 update[i] 的 level[i] 的 span 值为当前实例的总长度。更新每一层的索引后，更新 sl 的 level 为新的 level。

之后实例化了一个 Element 中存放着 v 数据，层级为 level。

循环每一层，将 x 中的每一层的前节点连接为 update[i] 的 level[i] 的前节点，将 update[i] 节点的 level[i] 节点的前节点都连接到 x。

将插入节点的每个 level 的 span 更新为其后节点的对应 level 的 span - rank[0] + rank[i]，更新后节点的对应 level 的 span 为 rank[0] - rank[i] + 1.

我们这里来整理一下此处的 span 和 rank。

rank 数组：

rank 为插入时的辅助数据，和 update 类似，update 会存储新增数据在各层的查找路径。rank 负责存储跨度。对于上面新增的层，赋值 rank[i] 为 0，对于第一层的已有层，赋值为从 header 到最后一个比 v 小的元素的 span 跨度之和，对于后面的层，则需要在 rank[i-1] 继续加上新的跨度，即 rank[i] 存储了在原实例中搜索新增数据位置，搜到第 i 层的总跨度。（可以结合下图来理解，其中的括号内的数字代表着每个数据存储的 span，即当前节点到同层下一节点，相对底层来说跨越的节点数）

![Untitled](%E8%B7%B3%E8%A1%A8%200daa641ce789462d9196883cd1c83f3f/Untitled%204.png)

结合上代码，在插入数据前，首先需要构造好 rank 数组。

span：

span 记录了当前节点到该层下一节点相对于底层链表而言，跨越的节点数，注意到 header nil 数据节点也是考虑在内的，所以第一个存储数据的节点的 span 即为 1 了。

因此在插入节点时需要更新相关节点的 span 值，即每层的 update[i] 节点的 span 值和每层新增的节点 x 的 span 值。每层 x 的 span 值即其前节点的 span 值 - (rank[0]-rank[i]) = update[i].level[i].span - rank[0] + rank[i]. 如下图所示：

![Untitled](%E8%B7%B3%E8%A1%A8%200daa641ce789462d9196883cd1c83f3f/Untitled%205.png)

而 update[i].level[i] 的新 span 值则为 rank[0] - rank[i] + 1（因为注意到这里的 rank[0] 其实是底层链表从 header 到 x 的前一个节点的 span，所以到 x 的 span 还需要增加 1；之前的 x 的 span 计算这里，其实它等于原来实例中 x 的前节点到 i 层下一节点的 span.

对于本次随机 level 之上的 level，则需要每层将 update[i].level[i].span 自增 1，因为相对于底层，这里新插入了一个数据。

参考文章：[解释 redis 跳表中 span 的作用](https://blog.csdn.net/qq_19648191/article/details/85381769)、[redis 源码学习-跳表篇](https://zhuanlan.zhihu.com/p/422537946)

接下来：

```go
if sl.update[0] == sl.header {
	x.backward = nil
} else {
	x.backward = sl.update[0]
}
if x.level[0].forward != nil {
	x.level[0].forward.backward = x
} else {
	sl.tail = x
}
sl.length++
```

对于插入数据为本 level 的第一个数据的，后节点为 nil，否则后节点为 update[0].

如果 x 的前节点不为空，那么其前节点的后节点为 x，否则 实例的 tail 节点为 x。

最后实例的存储数据 length 自增 1.

### 查找数据

了解完核心的插入数据，对于其他的操作就比较容易理解了。查找数据代码：

```go
// Find finds an element e that e.Value == v, and returns e or nil.
func (sl *SkipList) Find(v Interface) *Element {
	x := sl.find(v)                   // x.Value >= v
	if x != nil && !v.Less(x.Value) { // v >= x.Value
		return x
	}

	return nil
}
```

这里主要封装了内部实现使用的 find 方法，其如果找到返回第一个比当前数据在序关系上大于等于 v，在 Find 方法中，如果 v 小于了找到的 x，那么返回 nil。

对于 find 方法：

```go
// find finds the first element e that e.Value >= v, and returns e or nil.
func (sl *SkipList) find(v Interface) *Element {
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil && x.level[i].forward.Value.Less(v) {
			x = x.level[i].forward
		}
		sl.update[i] = x
	}

	return x.level[0].forward
}
```

x 定位从 header 开始，然后从最高 level 往下遍历，每层找到第一个前节点值不小于 v 的节点，即最后一个小于 v 的节点，然后到下一层。最终到 0 层，返回了 0 层最后一个小于 v 的节点的前节点（即等于 v 节点，或者大于 v 节点，或者为 nil）

SkipList 中对于查询相关还提供了两个接口。

第一个为：

```go
// GetRank finds the rank for an element e that e.Value == v,
// Returns 0 when the element cannot be found, rank otherwise.
// Note that the rank is 1-based due to the span of sl.header to the first element.
func (sl *SkipList) GetRank(v Interface) int {
	x := sl.header
	rank := 0
	for i := sl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil && x.level[i].forward.Value.Less(v) {
			rank += x.level[i].span
			x = x.level[i].forward
		}
		if x.level[i].forward != nil && !x.level[i].forward.Value.Less(v) && !v.Less(x.level[i].forward.Value) {
			rank += x.level[i].span
			return rank
		}
	}

	return 0
}
```

通过数据，拿到其下标，和  find 函数类似的进行查找，但是在查找过程中记录下所经过节点的 span 值并求和，存储在 rank 中，找到经过层的最后一个小于 v 的节点，在每一层判断该节点的前节点是否和 v 相等，如果相等，那么在 rank 中再加上该最后小于 v 节点的 span 值，得到总 rank 值并返回，即可得到在底层链表中 v 节点的下标。

```go
// GetElementByRank finds an element by ites rank. The rank argument needs bo be 1-based.
// Note that is the first element e that GetRank(e.Value) == rank, and returns e or nil.
func (sl *SkipList) GetElementByRank(rank int) *Element {
	x := sl.header
	traversed := 0
	for i := sl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil && traversed+x.level[i].span <= rank {
			traversed += x.level[i].span
			x = x.level[i].forward
		}
		if traversed == rank {
			return x
		}
	}

	return nil
}
```

另一方面，类似的，也可以通过 rank 即下标来进行访问得到数据。和 find 类似的过程，使用 traversed 变量存储经过的 rank 总长，每层循环到到下一节点的 rank 值大于目标 rank 时到下一层，当某层下一个节点的 span + 当前 rank 正好 等于目标 rank 时，找到目标并返回。

### 删除数据

删除数据提供了两个接口：

```go
// Remove removes e from sl if e is an element of skiplist sl.
// It returns the element value e.Value.
func (sl *SkipList) Remove(e *Element) interface{} {
	x := sl.find(e.Value)                 // x.Value >= e.Value
	if x == e && !e.Value.Less(x.Value) { // e.Value >= x.Value
		sl.deleteElement(x, sl.update)
		return x.Value
	}

	return nil
}

// Delete deletes an element e that e.Value == v, and returns e.Value or nil.
func (sl *SkipList) Delete(v Interface) interface{} {
	x := sl.find(v)                   // x.Value >= v
	if x != nil && !v.Less(x.Value) { // v >= x.Value
		sl.deleteElement(x, sl.update)
		return x.Value
	}

	return nil
}
```

分别为提供了节点指针和数据值。类似的通过 find 函数，传入有序关系的数据，找到数据，确认为需要删除的数据后，传入内部实现的 deleteElement 方法，传入的是 Element 指针当前的 sl.update 列表。在 find 过程中会把每一层最后一个小于 v 的节点存放在 update 中。

deleteElement 方法：

```go
// deleteElement deletes e from its skiplist, and decrements sl.length.
func (sl *SkipList) deleteElement(e *Element, update []*Element) {
	for i := 0; i < sl.level; i++ {
		if update[i].level[i].forward == e {
			update[i].level[i].span += e.level[i].span - 1
			update[i].level[i].forward = e.level[i].forward
		} else {
			update[i].level[i].span -= 1
		}
	}

	if e.level[0].forward != nil {
		e.level[0].forward.backward = e.backward
	} else {
		sl.tail = e.backward
	}

	for sl.level > 1 && sl.header.level[sl.level-1].forward == nil {
		sl.level--
	}
	sl.length--
}
```

在这里，首先遍历每一层，对于每一层的 update 节点的前节点，如果 == e，那么更新 update[i].level[i].span 值为 加上待删除节点 e 的 level[i].span 值 - 1. 同时将其前节点改为指向 e 的前节点；否则只需要将 update[i].level[i] 的 span 值 -1.

接着对于第 0 层，如果 e 的前节点不为 nil，那么需要把 e 的前节点的后节点改为指向 e 的后节点；否则将实例的 tail 改为 e 的后节点。

当实例的层数 level 大于 1，并且该层的 header 的前节点为 nil，那么将实例的 level 变量 -1（其实它的 level 数组的长度并没有变化，通过维护一个 level 变量，从而每次搜索不需要从默认的所有层中搜索，只需要搜索有索引的层。

最后对于实例 sl 的长度 length -1.