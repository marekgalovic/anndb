	.section	__TEXT,__text,regular,pure_instructions
	.build_version macos, 10, 15	sdk_version 10, 15
	.intel_syntax noprefix
	.globl	__Z26euclidean_distance_squaredmPfS_S_ ## -- Begin function _Z26euclidean_distance_squaredmPfS_S_
	.p2align	4, 0x90
__Z26euclidean_distance_squaredmPfS_S_: ## @_Z26euclidean_distance_squaredmPfS_S_
## %bb.0:
	push	rbp
	mov	rbp, rsp
	and	rsp, -8
	test	rdi, rdi
	je	LBB0_1
## %bb.2:
	dec	rdi
	mov	r9, rdi
	shr	r9, 3
	lea	r8d, [r9 + 1]
	and	r8d, 3
	cmp	rdi, 24
	jae	LBB0_8
## %bb.3:
	vxorps	xmm0, xmm0, xmm0
	xor	edi, edi
	test	r8, r8
	jne	LBB0_5
	jmp	LBB0_7
LBB0_1:
	vxorps	xmm0, xmm0, xmm0
	jmp	LBB0_7
LBB0_8:
	lea	rax, [r8 - 1]
	sub	rax, r9
	vxorps	xmm0, xmm0, xmm0
	xor	edi, edi
	.p2align	4, 0x90
LBB0_9:                                 ## =>This Inner Loop Header: Depth=1
	vmovups	ymm1, ymmword ptr [rsi + 4*rdi]
	vmovups	ymm2, ymmword ptr [rsi + 4*rdi + 32]
	vmovups	ymm3, ymmword ptr [rsi + 4*rdi + 64]
	vmovups	ymm4, ymmword ptr [rsi + 4*rdi + 96]
	vsubps	ymm1, ymm1, ymmword ptr [rdx + 4*rdi]
	vmulps	ymm1, ymm1, ymm1
	vaddps	ymm0, ymm0, ymm1
	vsubps	ymm1, ymm2, ymmword ptr [rdx + 4*rdi + 32]
	vmulps	ymm1, ymm1, ymm1
	vaddps	ymm0, ymm0, ymm1
	vsubps	ymm1, ymm3, ymmword ptr [rdx + 4*rdi + 64]
	vmulps	ymm1, ymm1, ymm1
	vaddps	ymm0, ymm0, ymm1
	vsubps	ymm1, ymm4, ymmword ptr [rdx + 4*rdi + 96]
	vmulps	ymm1, ymm1, ymm1
	vaddps	ymm0, ymm0, ymm1
	add	rdi, 32
	add	rax, 4
	jne	LBB0_9
## %bb.4:
	test	r8, r8
	je	LBB0_7
LBB0_5:
	shl	rdi, 2
	neg	r8
	.p2align	4, 0x90
LBB0_6:                                 ## =>This Inner Loop Header: Depth=1
	vmovups	ymm1, ymmword ptr [rsi + rdi]
	vsubps	ymm1, ymm1, ymmword ptr [rdx + rdi]
	vmulps	ymm1, ymm1, ymm1
	vaddps	ymm0, ymm0, ymm1
	add	rdi, 32
	inc	r8
	jne	LBB0_6
LBB0_7:
	vhaddps	ymm0, ymm0, ymm0
	vhaddps	ymm0, ymm0, ymm0
	vextractf128	xmm1, ymm0, 1
	vaddss	xmm0, xmm0, xmm1
	vmovss	dword ptr [rcx], xmm0
	mov	rsp, rbp
	pop	rbp
	vzeroupper
	ret
                                        ## -- End function
	.globl	__Z18manhattan_distancemPfS_S_ ## -- Begin function _Z18manhattan_distancemPfS_S_
	.p2align	4, 0x90
__Z18manhattan_distancemPfS_S_:         ## @_Z18manhattan_distancemPfS_S_
## %bb.0:
	push	rbp
	mov	rbp, rsp
	and	rsp, -8
	test	rdi, rdi
	je	LBB1_1
## %bb.2:
	dec	rdi
	shr	rdi, 3
	lea	r8d, [rdi + 1]
	and	r8d, 1
	test	rdi, rdi
	je	LBB1_3
## %bb.7:
	lea	rax, [r8 - 1]
	sub	rax, rdi
	vxorps	xmm0, xmm0, xmm0
	xor	edi, edi
	.p2align	4, 0x90
LBB1_8:                                 ## =>This Inner Loop Header: Depth=1
	vmovups	ymm1, ymmword ptr [rsi + 4*rdi]
	vsubps	ymm1, ymm1, ymmword ptr [rdx + 4*rdi]
	vmulps	ymm1, ymm1, ymm1
	vsqrtps	ymm1, ymm1
	vmovups	ymm2, ymmword ptr [rsi + 4*rdi + 32]
	vsubps	ymm2, ymm2, ymmword ptr [rdx + 4*rdi + 32]
	vmulps	ymm2, ymm2, ymm2
	vsqrtps	ymm2, ymm2
	vaddps	ymm0, ymm0, ymm1
	vaddps	ymm0, ymm0, ymm2
	add	rdi, 16
	add	rax, 2
	jne	LBB1_8
## %bb.4:
	test	r8, r8
	jne	LBB1_5
	jmp	LBB1_6
LBB1_1:
	vxorps	xmm0, xmm0, xmm0
	jmp	LBB1_6
LBB1_3:
	vxorps	xmm0, xmm0, xmm0
	xor	edi, edi
	test	r8, r8
	je	LBB1_6
LBB1_5:
	vmovups	ymm1, ymmword ptr [rsi + 4*rdi]
	vsubps	ymm1, ymm1, ymmword ptr [rdx + 4*rdi]
	vmulps	ymm1, ymm1, ymm1
	vsqrtps	ymm1, ymm1
	vaddps	ymm0, ymm0, ymm1
LBB1_6:
	vhaddps	ymm0, ymm0, ymm0
	vhaddps	ymm0, ymm0, ymm0
	vextractf128	xmm1, ymm0, 1
	vaddss	xmm0, xmm0, xmm1
	vmovss	dword ptr [rcx], xmm0
	mov	rsp, rbp
	pop	rbp
	vzeroupper
	ret
                                        ## -- End function
	.globl	__Z26cosine_similarity_dot_normmPfS_S_S_ ## -- Begin function _Z26cosine_similarity_dot_normmPfS_S_S_
	.p2align	4, 0x90
__Z26cosine_similarity_dot_normmPfS_S_S_: ## @_Z26cosine_similarity_dot_normmPfS_S_S_
## %bb.0:
	push	rbp
	mov	rbp, rsp
	and	rsp, -8
	test	rdi, rdi
	je	LBB2_1
## %bb.2:
	dec	rdi
	shr	rdi, 3
	lea	r9d, [rdi + 1]
	and	r9d, 1
	test	rdi, rdi
	je	LBB2_3
## %bb.7:
	lea	rax, [r9 - 1]
	sub	rax, rdi
	vxorps	xmm0, xmm0, xmm0
	xor	edi, edi
	vxorps	xmm1, xmm1, xmm1
	vxorps	xmm2, xmm2, xmm2
	.p2align	4, 0x90
LBB2_8:                                 ## =>This Inner Loop Header: Depth=1
	vmovups	ymm3, ymmword ptr [rsi + 4*rdi]
	vmovups	ymm4, ymmword ptr [rsi + 4*rdi + 32]
	vmovups	ymm5, ymmword ptr [rdx + 4*rdi]
	vmovups	ymm6, ymmword ptr [rdx + 4*rdi + 32]
	vmulps	ymm7, ymm3, ymm5
	vaddps	ymm2, ymm2, ymm7
	vmulps	ymm3, ymm3, ymm3
	vaddps	ymm1, ymm1, ymm3
	vmulps	ymm3, ymm5, ymm5
	vaddps	ymm0, ymm0, ymm3
	vmulps	ymm3, ymm4, ymm6
	vaddps	ymm2, ymm2, ymm3
	vmulps	ymm3, ymm4, ymm4
	vaddps	ymm1, ymm1, ymm3
	vmulps	ymm3, ymm6, ymm6
	vaddps	ymm0, ymm0, ymm3
	add	rdi, 16
	add	rax, 2
	jne	LBB2_8
## %bb.4:
	test	r9, r9
	jne	LBB2_5
	jmp	LBB2_6
LBB2_1:
	vxorps	xmm2, xmm2, xmm2
	vxorps	xmm1, xmm1, xmm1
	vxorps	xmm0, xmm0, xmm0
	jmp	LBB2_6
LBB2_3:
	vxorps	xmm0, xmm0, xmm0
	xor	edi, edi
	vxorps	xmm1, xmm1, xmm1
	vxorps	xmm2, xmm2, xmm2
	test	r9, r9
	je	LBB2_6
LBB2_5:
	vmovups	ymm3, ymmword ptr [rsi + 4*rdi]
	vmovups	ymm4, ymmword ptr [rdx + 4*rdi]
	vmulps	ymm5, ymm4, ymm4
	vaddps	ymm0, ymm0, ymm5
	vmulps	ymm5, ymm3, ymm3
	vaddps	ymm1, ymm1, ymm5
	vmulps	ymm3, ymm3, ymm4
	vaddps	ymm2, ymm2, ymm3
LBB2_6:
	vhaddps	ymm2, ymm2, ymm2
	vhaddps	ymm2, ymm2, ymm2
	vextractf128	xmm3, ymm2, 1
	vaddss	xmm2, xmm2, xmm3
	vhaddps	ymm1, ymm1, ymm1
	vmovss	dword ptr [rcx], xmm2
	vhaddps	ymm1, ymm1, ymm1
	vextractf128	xmm2, ymm1, 1
	vhaddps	ymm0, ymm0, ymm0
	vaddss	xmm1, xmm1, xmm2
	vhaddps	ymm0, ymm0, ymm0
	vextractf128	xmm2, ymm0, 1
	vaddss	xmm0, xmm0, xmm2
	vmulss	xmm0, xmm1, xmm0
	vmovss	dword ptr [r8], xmm0
	mov	rsp, rbp
	pop	rbp
	vzeroupper
	ret
                                        ## -- End function

.subsections_via_symbols
