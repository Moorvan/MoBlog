---
tags:
- Go

title: "Go 语言中 map 的 key 值类型 & 不可取址 (Not Addressable)"
date: 2022-11-25T23:33:24+08:00
draft: true
---

# 写在前面

(可跳过)

> invalid operation: cannot take address of xxx.
>

在做一道算法题时候，构造了一个字典树，之前看到在 Go 中构造字典树只需要定义：

```go
type trie map[byte]trie
```

这个类型有点类似于递归定义了。但是这里利用了 Go 中的内置 map 其实是指针类型，因此在初始化得到一个 trie 实例后，其所有的子节点都是 null。

```go
type node struct {
	v int
	t node
}
// invalid recursive type trie
```

对于上面的递归定义实际上是会报错的。

类似的定义成这样就是合法的：

```go
type list struct {
	v int
	t *list
}
```

就是一个链表了。

由此可见，利用内置的 map 定义一个字典树就十分优雅了。但是在这题中，我考虑到测试样例可能会出现重复的单词，因此需要对每个单词在字典树中结束的节点进行计数，使用了一个：

```go
cnt := make(map[trie]int)
// invalid map key type trie
// Invalid map key type: comparison operators == and != must be fully defined for the key type
```

所以 map 类型不能作为 map 的 key 类型（虽然普通的指针是可以的），因为其不是 Comparable 类型）。

那么我想将 map 放到一个内存中，使用一个指针来作为 map 的 key 类型总归是可以的，即：

```go
type trie map[byte]trie
cnt := make(map[*trie]int)
root := make(trie)
cnt[&root] = 1
```

可以看到，到这里都是编译可以的。

然而：

```go
root['a'] = make(trie)
cnt[&root['a']] = 1
// Cannot take the address of 'root['a']'
```

出现问题了，map 的 value 值是不可取地址的。

一不做二不休，全用指针就可以解决问题了，但是解指针运算需要嵌套，这里包了几个函数：

```go
type trie *map[byte]trie

func newTrie() trie {
	t := make(map[byte]trie)
	return &t
}

func add(t trie, i byte) (cur trie) {
	(*t)[i] = newTrie()
	return (*t)[i]
}

func get(t trie, i byte) trie {
	return (*t)[i]
}

func main() {
	cnt := make(map[trie]int)
	root := newTrie()
	cur := add(root, 'a')
	cur = add(cur, 'b')
	cnt[cur]++
	println(cnt[get(get(root, 'a'), 'b')])
}
```

可以满足最开始的需求了，但是就一点都不优雅了。

这里涉及到几个问题，因此在这里梳理一下。

# map 的 key 类型

可以看到，作为 map 的 key 值，必须要求类型是 Comparable 的，这里准确来说是要求 Hashable，需要可以通过运算符 == 和 != 比较。

在 Go 规范中，将可应用 == 和 != 比较的定义为 Comparable，而对于同时拥有序关系定义的（可应用 >, < 等比较运算符）的定义为 ordered.

在官方规范中，定义为：

> These terms and the result of the comparisons are defined as follows:
>
> - Boolean values are comparable. Two boolean values are equal if they are either both `true` or both `false`.
> - Integer values are comparable and ordered, in the usual way.
> - Floating-point values are comparable and ordered, as defined by the IEEE-754 standard.
> - Complex values are comparable. Two complex values `u` and `v` are equal if both `real(u) == real(v)` and `imag(u) == imag(v)`.
> - String values are comparable and ordered, lexically byte-wise.
> - Pointer values are comparable. Two pointer values are equal if they point to the same variable or if both have value `nil`. Pointers to distinct [zero-size](https://go.dev/ref/spec#Size_and_alignment_guarantees)variables may or may not be equal.
> - Channel values are comparable. Two channel values are equal if they were created by the same call to `[make](https://go.dev/ref/spec#Making_slices_maps_and_channels)` or if both have value `nil`.
> - Interface values are comparable. Two interface values are equal if they have [identical](https://go.dev/ref/spec#Type_identity) dynamic types and equal dynamic values or if both have value `nil`.
> - A value `x` of non-interface type `X` and a value `t` of interface type `T` are comparable when values of type `X` are comparable and `X` [implements](https://go.dev/ref/spec#Implementing_an_interface) `T`. They are equal if `t`'s dynamic type is identical to `X` and `t`'s dynamic value is equal to `x`.
> - Struct values are comparable if all their fields are comparable. Two struct values are equal if their corresponding non-[blank](https://go.dev/ref/spec#Blank_identifier) fields are equal.
> - Array values are comparable if values of the array element type are comparable. Two array values are equal if their corresponding elements are equal.

即：（ c 代表仅 comparable，co 代表 ordered&comparable）

1. bool 类型；co
2. 浮点类型；co
3. 复数类型；c
4. string；co
5. 指针；c，如果相等代表指向同一块资源或者都是 nil
6. chan；c
7. interface value；c
8. non-interface type X 的 value x 和 interface type T 的 value t，且 X 是 comparable 且 X 实现了 T；c，相等 or nil
9. struct value 如果他们的 fields 都是 comparable；c
10. array；c，对应元素相等。

可惜 map 虽然是地址，但是并不是 comparable 的。

# 不可取址类型

## Go 中都是值类型

不同于 C++，在 Go 语言中，实际上并没有引用类型。在 C++ 中，在传参数或者可以直接拿到一个变量的引用，这里的引用类似于别名。然而在 Go 中，似乎是在设计上有了 C++ PTSD，更多采用了 C 的思想，Go 中每个变量都维护着独一无二的一块内存空间。可以通过下面的代码证明（引自 [There is no pass-by-reference in Go](https://dave.cheney.net/2017/04/29/there-is-no-pass-by-reference-in-go)）：

```go
package main

import "fmt"

func fn(m map[int]int) {
    m = make(map[int]int)
}

func main() {
    var m map[int]int
    fn(m)
    fmt.Println(m == nil)
}
// Console: m == nil
```

上面的代码执行结果证明了即使是 map 维护的也只是一个指针，而并非引用。

可以理解为，究根结底而言，Go 语言中都是值类型，只是有可能这个值的内容是一个内存地址。这样需要在多处维护同一个内存资源时，就只能进行取址。即对变量执行 & 操作，除了：

```go
v := &X{}
```

这个操作类似于 new(X).

## 可取址 & 不可取址

这里是一份对于可取址和不可取址的列举，参考文章：[Go 中的可寻址和不可寻址怎么理解？](https://www.51cto.com/article/684755.html)

### 可取址

1. 变量
2. 指针
3. 数组元素
4. 切片元素

### 不可取址

1. 所有的字面量
    1. 简单类型
    2. 复合类型
    3. 数组
2. 常量
3. 函数 & 方法
4. **map 中的元素**
5. interface value

引自 Golang wiki 的 [MethodSets](https://github.com/golang/go/wiki/MethodSets#interfaces) 有：

> The concrete value stored in an interface is not addressable, in the same way that a map element is not addressable.
>

对于 map 的 value 值不可取址，原因简单分析有二：

1. 对于 map 的该元素不存在，那么 map 会返回 0 值，那么其取址就意义不明了；
2. map 中的元素地址是会变化的，所以地址没有意义。

这里对于 map 的 v 值不可取址就可以理解了。

# 扩展

由此想到一些其他问题。

## slice 类型取址问题

map v 值不能取址的一个原因在于 map 的元素位置可能会发生变化，类似的，Go 中 slice 在生命中可能会隐式扩容，因此元素的地址也发生了变化，那么之前使用的引用的位置的内容就不再是 slice 中的内容了（虽然由于 slice 的设定问题，也会有很多类似的其他问题），例如以下代码：

```go
func main() {
	ar := make([]int, 3, 4)
	ar2 := &ar[2]
	println(ar[2]) // init: 0
	*ar2 = 1
	println(ar[2]) // 1
	ar = append(ar, 2)
	*ar2 = 19
	println(ar[2]) // 19
	ar = append(ar, 3)
	*ar2 = 20
	println(ar[2]) // 19, 发生了扩容，ar2 不再可以修改 slice 中的内容
}
```

虽然这个问题在了解 slice 的底层原理后就可以理解，但是这里的取址引用可能会发生意想不到的错误吧。（回头写一篇关于 Go slice 设计导致的一些反直觉的问题）

## 值 or 指针 实现了接口

这个问题之前都是强行理解，但是在接口无法取址这个设定下，就可以很好的理解了。对于下面的代码片段：（引用自 [Golang 不可寻址的理解](http://www.wu.run/2021/11/12/not-addressable-in-golang/)）

```go
package main

type Person interface {
	getName() string
	setName(name string)
}

type Male struct {
	Name string
}

func (m Male) getName() string {
	return m.Name
}

func (m *Male) setName(name string) {
	m.Name = name
}

func main() {
	var p1 Person = Male{} // error
	p1.getName()
	p1.setName("XiaoMin")

	var p2 Person = &Male{}
	p2.getName()
	p2.setName("XiaoMin")
}
```

在这个例子中，我以前是记住，指针形式是更加高级的，所以这里是 Male 指针实现了该接口，Male 值类型并没有实现该接口，所以 Male{} 并不能转为 Person 类型。但是这里 Male 实例也是可以调用 getName() 和 setName() 两个方法。

实际上需要这么理解，这里在调用 getName() 方法时，对于指针类型，则需要取值，调用 setName() 方法，对于值类型，需要取址才能调用。在 Go 中，在没有接口的情况下，Go 可以隐式地在值和指针类型之间自动转化，但是当转为 Person 接口类型，接口是无法取址的，因此对于值类型如果转成了 Person 类型后，就没办法取址来调用 setName 方法，因此即，值类型 Male 没有实现 Person 接口。